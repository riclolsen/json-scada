package serverapp

import "go.mongodb.org/mongo-driver/v2/bson"

// toBson converts a plain map fixture into the bson.M/bson.A shapes the driver
// receives from the MongoDB driver, so the tests exercise the same accessors
// production does.
func toBson(m map[string]any) bson.M {
	out := bson.M{}
	for k, v := range m {
		out[k] = toBsonValue(v)
	}
	return out
}

func toBsonValue(v any) any {
	switch x := v.(type) {
	case map[string]any:
		return toBson(x)
	case []any:
		arr := make(bson.A, 0, len(x))
		for _, el := range x {
			arr = append(arr, toBsonValue(el))
		}
		return arr
	default:
		return v
	}
}

func toBsonSlice(ms []map[string]any) []bson.M {
	out := make([]bson.M, 0, len(ms))
	for _, m := range ms {
		out = append(out, toBson(m))
	}
	return out
}
