/*
 * OPC-UA Client Protocol driver for {json:scada}, in Go.
 * {json:scada} - Copyright (c) 2020-2026 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 *
 * This program is free software: you can redistribute it and/or modify
 * it under the terms of the GNU General Public License as published by
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but
 * WITHOUT ANY WARRANTY; without even the implied warranty of
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

// OPC UA value conversion. Port of ConvertOpcValue() in
// AsduReceiveHandler.cs.
//
// The ladder of type tests below is deliberately in the same order as the
// C# one: the branches overlap (xmlelement is matched twice, arrays of the
// string family before arrays in general), so reordering them changes what
// lands in the database.

package main

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"time"

	"github.com/gopcua/opcua/ua"
)

// builtInNames maps gopcua's type ids to the lowercased names of the C#
// BuiltInType enum, which is what protocolSourceASDU holds.
var builtInNames = map[ua.TypeID]string{
	ua.TypeIDNull:            "null",
	ua.TypeIDBoolean:         "boolean",
	ua.TypeIDSByte:           "sbyte",
	ua.TypeIDByte:            "byte",
	ua.TypeIDInt16:           "int16",
	ua.TypeIDUint16:          "uint16",
	ua.TypeIDInt32:           "int32",
	ua.TypeIDUint32:          "uint32",
	ua.TypeIDInt64:           "int64",
	ua.TypeIDUint64:          "uint64",
	ua.TypeIDFloat:           "float",
	ua.TypeIDDouble:          "double",
	ua.TypeIDString:          "string",
	ua.TypeIDDateTime:        "datetime",
	ua.TypeIDGUID:            "guid",
	ua.TypeIDByteString:      "bytestring",
	ua.TypeIDXMLElement:      "xmlelement",
	ua.TypeIDNodeID:          "nodeid",
	ua.TypeIDExpandedNodeID:  "expandednodeid",
	ua.TypeIDStatusCode:      "statuscode",
	ua.TypeIDQualifiedName:   "qualifiedname",
	ua.TypeIDLocalizedText:   "localizedtext",
	ua.TypeIDExtensionObject: "extensionobject",
	ua.TypeIDDataValue:       "datavalue",
	ua.TypeIDVariant:         "variant",
	ua.TypeIDDiagnosticInfo:  "diagnosticinfo",
}

// stringFamily are the types the C# driver renders as text rather than as a
// number. localeid and utctime are C#-only BuiltInType members that gopcua
// has no equivalent for; they are kept so the two drivers agree on the same
// set of names.
var stringFamily = map[string]bool{
	"string":        true,
	"localeid":      true,
	"localizedtext": true,
	"xmlelement":    true,
	"qualifiedname": true,
	"guid":          true,
}

// marshalJSON renders a value as JSON without Go's default HTML escaping.
//
// encoding/json escapes '<', '>' and '&' into their six-character unicode
// forms, which would turn every XML element and every string holding those
// characters into unreadable output in valueJsonAtSource. The result is
// still valid JSON either way; this just keeps it legible.
func marshalJSON(v any) (string, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	if err := enc.Encode(v); err != nil {
		return "", err
	}
	// Encode appends a newline that Marshal does not.
	return strings.TrimRight(buf.String(), "\n"), nil
}

// statusIsGood reports whether a status code carries a usable value.
//
// gopcua has no equivalent of C#'s StatusCode.IsGood. Bits 30-31 are the
// severity: 00 good, 01 uncertain, 10/11 bad. Uncertain is not good, which
// is what the C# helper says too.
func statusIsGood(c ua.StatusCode) bool {
	return c&0xC0000000 == 0
}

// convertOPCValue renders one DataValue the way the C# driver does.
//
// tp becomes protocolSourceASDU, dbl becomes valueAtSource, str becomes
// valueStringAtSource and jsonStr becomes valueJsonAtSource.
func convertOPCValue(dv *ua.DataValue) (tp string, dbl float64, str string, jsonStr string, isArray bool) {
	tp = "unknown"

	// parity: the C# function leaves every output at its default when
	// there is no value at all.
	if dv == nil || dv.Value == nil || dv.Value.Value() == nil {
		return tp, 0, "", "", false
	}

	v := dv.Value
	raw := v.Value()

	base, known := builtInNames[v.Type()]
	if !known {
		base = "unknown"
	}
	isArray = v.Has(ua.VariantArrayValues)
	tp = base
	if isArray {
		// TypeInfo.ToString() renders rank 1 as "double[]" and rank 2 as
		// "double[,]".
		rank := 1
		if v.Has(ua.VariantArrayDimensions) && len(v.ArrayDimensions()) > 0 {
			rank = len(v.ArrayDimensions())
		}
		tp = base + "[" + strings.Repeat(",", rank-1) + "]"
	}

	if s, err := marshalJSON(jsonRenderable(raw)); err == nil {
		jsonStr = s
	}

	switch {
	case base == "variant" && !isArray:
		// A variant nested in a variant: try progressively narrower
		// numeric conversions, then give up and keep the JSON.
		if f, ok := numericOf(unwrapVariant(raw)); ok {
			dbl = f
		} else {
			dbl = 0
			str = jsonStr
		}

	case (base == "datetime" || base == "utctime") && !isArray:
		if t, ok := raw.(time.Time); ok {
			dbl = float64(t.UnixMilli())
			str = t.Format(time.RFC3339Nano)
		}

	case base == "extensionobject" && !isArray:
		dbl = 0
		// The C# driver strips TypeId/BinaryEncodingId/XmlEncodingId/
		// JsonEncodingId out of the serialized body. gopcua hands over the
		// decoded body without those fields, so rendering it is enough.
		if eo, ok := raw.(*ua.ExtensionObject); ok && eo != nil {
			if s, err := marshalJSON(jsonRenderable(eo.Value)); err == nil {
				str = s
			} else {
				str = jsonStr
			}
		} else {
			str = jsonStr
		}

	case base == "nodeid" && !isArray:
		dbl = 0
		str = nodeIDString(raw)

	case base == "expandednodeid" && !isArray:
		dbl = 0
		str = nodeIDString(raw)

	case base == "xmlelement" && !isArray:
		dbl = 0
		str = stringOf(raw)
		// Serialized as a JSON string so quotes and backslashes inside the
		// XML stay valid JSON.
		if s, err := marshalJSON(str); err == nil {
			jsonStr = s
		}

	case base == "bytestring" && !isArray:
		dbl = 0
		str = base64.StdEncoding.EncodeToString(byteStringOf(raw))

	case stringFamily[base] && isArray:
		dbl = 0
		str = jsonStr

	case stringFamily[base]:
		dbl = 0
		str = stringOf(raw)

	case isArray:
		dbl = 0
		str = jsonStr

	default:
		if f, ok := numericOf(raw); ok {
			dbl = f
			str = stringOf(raw)
		} else {
			dbl = 0
			str = stringOf(raw)
		}
	}

	return tp, dbl, str, jsonStr, isArray
}

// unwrapVariant peels nested variants down to the value they carry.
func unwrapVariant(v any) any {
	for {
		inner, ok := v.(*ua.Variant)
		if !ok || inner == nil {
			return v
		}
		v = inner.Value()
	}
}

// numericOf converts the numeric built-in types to a float64, reporting
// whether the value was numeric at all. Booleans count as numbers, as they
// do for C#'s Convert.ToDouble.
func numericOf(v any) (float64, bool) {
	switch t := v.(type) {
	case bool:
		if t {
			return 1, true
		}
		return 0, true
	case float32:
		return float64(t), true
	case float64:
		return t, true
	case time.Time:
		return float64(t.UnixMilli()), true
	}

	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64:
		return float64(rv.Int()), true
	case reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return float64(rv.Uint()), true
	case reflect.Float32, reflect.Float64:
		return rv.Float(), true
	}
	return 0, false
}

// stringOf renders a scalar the way C#'s Convert.ToString does for the
// corresponding Opc.Ua type.
func stringOf(v any) string {
	switch t := v.(type) {
	case nil:
		return ""
	case string:
		return t
	case ua.XMLElement:
		return string(t)
	case *ua.LocalizedText:
		if t == nil {
			return ""
		}
		return t.Text
	case *ua.QualifiedName:
		if t == nil {
			return ""
		}
		if t.NamespaceIndex != 0 {
			return fmt.Sprintf("%d:%s", t.NamespaceIndex, t.Name)
		}
		return t.Name
	case *ua.GUID:
		if t == nil {
			return ""
		}
		return t.String()
	case *ua.NodeID:
		return nodeIDString(t)
	case *ua.ExpandedNodeID:
		return nodeIDString(t)
	case ua.StatusCode:
		return statusCodeName(t)
	case []byte:
		return base64.StdEncoding.EncodeToString(t)
	case ua.ByteArray:
		return base64.StdEncoding.EncodeToString(t)
	case time.Time:
		return t.Format(time.RFC3339Nano)
	case bool:
		// C# renders booleans capitalised.
		if t {
			return "True"
		}
		return "False"
	}
	return fmt.Sprint(v)
}

// statusCodeName renders a status code the way Opc.Ua.StatusCode.ToString
// does: the symbolic name, "Good" for success. gopcua's own Error() adds
// the description text and the hex code, which would not match.
func statusCodeName(c ua.StatusCode) string {
	if c == ua.StatusOK {
		return "Good"
	}
	if d, ok := ua.StatusCodes[c]; ok && d.Name != "" {
		return strings.TrimPrefix(d.Name, "Status")
	}
	return fmt.Sprintf("0x%X", uint32(c))
}

func nodeIDString(v any) string {
	switch t := v.(type) {
	case *ua.NodeID:
		if t == nil {
			return ""
		}
		return t.String()
	case *ua.ExpandedNodeID:
		if t == nil {
			return ""
		}
		return t.String()
	}
	return fmt.Sprint(v)
}

func byteStringOf(v any) []byte {
	switch t := v.(type) {
	case []byte:
		return t
	case ua.ByteArray:
		return t
	}
	return nil
}

// jsonRenderable maps the OPC UA types onto values that marshal to stable,
// readable JSON. Marshalling the gopcua structs directly would leak wire
// details such as EncodingMask into the database.
//
// deviation D6: the JSON of structured types does not match what
// System.Text.Json produces for the corresponding .NET types. Scalars,
// strings, byte strings (base64) and arrays are identical.
func jsonRenderable(v any) any {
	switch t := v.(type) {
	case nil:
		return nil
	case *ua.LocalizedText:
		if t == nil {
			return nil
		}
		return map[string]any{"Locale": t.Locale, "Text": t.Text}
	case *ua.QualifiedName:
		if t == nil {
			return nil
		}
		return map[string]any{"NamespaceIndex": t.NamespaceIndex, "Name": t.Name}
	case *ua.NodeID:
		return nodeIDString(t)
	case *ua.ExpandedNodeID:
		return nodeIDString(t)
	case *ua.GUID:
		if t == nil {
			return nil
		}
		return t.String()
	case ua.XMLElement:
		return string(t)
	case ua.StatusCode:
		return uint32(t)
	case *ua.ExtensionObject:
		if t == nil {
			return nil
		}
		return jsonRenderable(t.Value)
	case *ua.Variant:
		if t == nil {
			return nil
		}
		return jsonRenderable(t.Value())
	case *ua.DataValue:
		if t == nil {
			return nil
		}
		return jsonRenderable(t.Value)
	case *ua.DiagnosticInfo:
		if t == nil {
			return nil
		}
		return fmt.Sprint(t)
	case ua.ByteArray:
		// Same as []byte: base64, which is also what System.Text.Json does.
		return []byte(t)
	case []byte:
		return t
	}

	// Arrays of anything else are rendered element by element.
	rv := reflect.ValueOf(v)
	if rv.Kind() == reflect.Slice || rv.Kind() == reflect.Array {
		out := make([]any, rv.Len())
		for i := 0; i < rv.Len(); i++ {
			out[i] = jsonRenderable(rv.Index(i).Interface())
		}
		return out
	}
	return v
}
