#region Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved
//-----------------------------------------------------------------------------
// Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved
// Web: https://www.technosoftware.com 
// 
// The source code in this file is covered under a dual-license scenario:
//   - Owner of a purchased license: SCLA 1.0
//   - GPL V3: everybody else
//
// SCLA license terms accompanied with this source code.
// See SCLA 1.0: https://technosoftware.com/license/Source_Code_License_Agreement.pdf
//
// GNU General Public License as published by the Free Software Foundation;
// version 3 of the License are accompanied with this source code.
// See https://technosoftware.com/license/GPLv3License.txt
//
// This source code is distributed in the hope that it will be useful, but
// WITHOUT ANY WARRANTY; without even the implied warranty of MERCHANTABILITY
// or FITNESS FOR A PARTICULAR PURPOSE.
//-----------------------------------------------------------------------------
#endregion Copyright (c) 2011-2023 Technosoftware GmbH. All rights reserved

#region Using Directives
using System;
using System.Collections;
#endregion

namespace Technosoftware.DaAeHdaClient.Cpx
{
	/// <summary>
	/// A class that reads a complex data item from a binary buffer.
	/// </summary>
	public class TsCCpxBinaryReader : TsCCpxBinaryStream
	{
		///////////////////////////////////////////////////////////////////////
		#region Constructors, Destructor, Initialization

		/// <summary>
		/// Initializes the reader with defaults.
		/// </summary>
		public TsCCpxBinaryReader() { }

		#endregion

		///////////////////////////////////////////////////////////////////////
		#region Public Methods

		/// <summary>
		/// Reads a value of the specified type from the buffer.
		/// </summary>
		/// <param name="buffer">The buffer containing binary data to read.</param>
		/// <param name="dictionary">The type dictionary that contains a complex type identified with the type name.</param>
		/// <param name="typeName">The name of the type that describes the data.</param>
		/// <returns>A structured represenation of the data in the buffer.</returns>
		public TsCCpxComplexValue Read(byte[] buffer, TypeDictionary dictionary, string typeName)
		{
			if (buffer == null) throw new ArgumentNullException(nameof(buffer));
			if (dictionary == null) throw new ArgumentNullException(nameof(dictionary));
			if (typeName == null) throw new ArgumentNullException(nameof(typeName));

			var context = InitializeContext(buffer, dictionary, typeName);

            TsCCpxComplexValue complexValue;
            var bytesRead = ReadType(context, out complexValue);

			if (bytesRead == 0)
			{
				throw new TsCCpxInvalidSchemaException(string.Format("Type '{0}' not found in dictionary.", typeName));
			}

			return complexValue;
		}

		#endregion

		///////////////////////////////////////////////////////////////////////
		#region Private Methods

		/// <summary>
		/// Reads an instance of a type from the buffer,
		/// </summary>
		private int ReadType(TsCCpxContext context, out TsCCpxComplexValue complexValue)
		{
			complexValue = null;

			var type = context.Type;
			var startIndex = context.Index;

			byte bitOffset = 0;

			var fieldValues = new ArrayList();

			for (var ii = 0; ii < type.Field.Length; ii++)
			{
				var field = type.Field[ii];

				var fieldValue = new TsCCpxComplexValue { Name = (field.Name != null && field.Name.Length != 0) ? field.Name : string.Format("[{0}]", ii), Type = null, Value = null };

				// check if additional padding is required after the end of a bit field.
				if (bitOffset != 0)
				{
					if (field.GetType() != typeof(BitString))
					{
						context.Index++;
						bitOffset = 0;
					}
				}

                int bytesRead;
                if (IsArrayField(field))
				{
					bytesRead = ReadArrayField(context, field, ii, fieldValues, out fieldValue.Value);
				}
				else if (field.GetType() == typeof(TypeReference))
				{
                    object typeValue;
                    bytesRead = ReadField(context, (TypeReference)field, out typeValue);

					// assign a name appropriate for the current context.
					fieldValue.Name = field.Name;
					fieldValue.Type = ((TsCCpxComplexValue)typeValue).Type;
					fieldValue.Value = ((TsCCpxComplexValue)typeValue).Value;
				}
				else
				{
					bytesRead = ReadField(context, field, ii, fieldValues, out fieldValue.Value, ref bitOffset);
				}

				if (bytesRead == 0 && bitOffset == 0)
				{
					throw new TsCCpxInvalidDataInBufferException(string.Format("Could not read field '{0}' in type '{1}'.", field.Name, type.TypeID));
				}

				context.Index += bytesRead;

				// assign a value for field type.
				if (fieldValue.Type == null)
				{
					fieldValue.Type = OpcConvert.ToString(fieldValue.Value.GetType());
				}

				fieldValues.Add(fieldValue);
			}

			// skip padding bits at the end of a type.
			if (bitOffset != 0)
			{
				context.Index++;
			}

			complexValue = new TsCCpxComplexValue();

			complexValue.Name = type.TypeID;
			complexValue.Type = type.TypeID;
			complexValue.Value = (TsCCpxComplexValue[])fieldValues.ToArray(typeof(TsCCpxComplexValue));

			return (context.Index - startIndex);
		}

		/// <summary>
		/// Reads the value contained in a field from the buffer.
		/// </summary>
		private int ReadField(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			ArrayList fieldValues,
			out object fieldValue,
			ref byte bitOffset
		)
		{
			fieldValue = null;

			var type = field.GetType();

			if (type == typeof(Integer) || type.IsSubclassOf(typeof(Integer)))
			{
				return ReadField(context, (Integer)field, out fieldValue);
			}
			else if (type == typeof(FloatingPoint) || type.IsSubclassOf(typeof(FloatingPoint)))
			{
				return ReadField(context, (FloatingPoint)field, out fieldValue);
			}
			else if (type == typeof(CharString) || type.IsSubclassOf(typeof(CharString)))
			{
				return ReadField(context, (CharString)field, fieldIndex, fieldValues, out fieldValue);
			}
			else if (type == typeof(BitString) || type.IsSubclassOf(typeof(BitString)))
			{
				return ReadField(context, (BitString)field, out fieldValue, ref bitOffset);
			}
			else if (type == typeof(TypeReference))
			{
				return ReadField(context, (TypeReference)field, out fieldValue);
			}
			else
			{
				throw new NotImplementedException(string.Format("Fields of type '{0}' are not implemented yet.", type));
			}
		}

		/// <summary>
		/// Reads a complex type from the buffer.
		/// </summary>
		private int ReadField(TsCCpxContext context, TypeReference field, out object fieldValue)
		{
            fieldValue = null;

			foreach (var type in context.Dictionary.TypeDescription)
			{
				if (type.TypeID == field.TypeID)
				{
					context.Type = type;

					if (type.DefaultBigEndianSpecified) context.BigEndian = type.DefaultBigEndian;
					if (type.DefaultCharWidthSpecified) context.CharWidth = type.DefaultCharWidth;
					if (type.DefaultStringEncoding != null) context.StringEncoding = type.DefaultStringEncoding;
					if (type.DefaultFloatFormat != null) context.FloatFormat = type.DefaultFloatFormat;

					break;
				}
			}

			if (context.Type == null)
			{
				throw new TsCCpxInvalidSchemaException(string.Format("Reference type '{0}' not found.", field.TypeID));
			}

            TsCCpxComplexValue complexValue;
            var bytesRead = ReadType(context, out complexValue);

			if (bytesRead == 0)
			{
				fieldValue = null;
			}
else
{

			fieldValue = complexValue;
}
			return bytesRead;
		}

		/// <summary>
		/// Reads a integer value from the buffer.
		/// </summary>
		private int ReadField(TsCCpxContext context, Integer field, out object fieldValue)
		{
			fieldValue = null;

			var buffer = context.Buffer;

			// initialize serialization paramters.
			var length = (field.LengthSpecified) ? (int)field.Length : 4;
			var signed = field.Signed;

			// apply defaults for built in types.
			if (field.GetType() == typeof(Int8)) { length = 1; signed = true; }
			else if (field.GetType() == typeof(Int16)) { length = 2; signed = true; }
			else if (field.GetType() == typeof(Int32)) { length = 4; signed = true; }
			else if (field.GetType() == typeof(Int64)) { length = 8; signed = true; }
			else if (field.GetType() == typeof(UInt8)) { length = 1; signed = false; }
			else if (field.GetType() == typeof(UInt16)) { length = 2; signed = false; }
			else if (field.GetType() == typeof(UInt32)) { length = 4; signed = false; }
			else if (field.GetType() == typeof(UInt64)) { length = 8; signed = false; }

			// check if there is enough data left.
			if (buffer.Length - context.Index < length)
			{
				throw new TsCCpxInvalidDataInBufferException("Unexpected end of buffer.");
			}

			// copy and swap bytes if required.
			var bytes = new byte[length];

			for (var ii = 0; ii < length; ii++)
			{
				bytes[ii] = buffer[context.Index + ii];
			}

			if (context.BigEndian)
			{
				SwapBytes(bytes, 0, length);
			}

			// convert to object.
			if (signed)
			{
				switch (length)
				{

					case 1:
						{
							if (bytes[0] < 128)
							{
								fieldValue = (sbyte)bytes[0];
							}
							else
							{
								fieldValue = (sbyte)(0 - bytes[0]);
							}

							break;
						}

					case 2: { fieldValue = BitConverter.ToInt16(bytes, 0); break; }
					case 4: { fieldValue = BitConverter.ToInt32(bytes, 0); break; }
					case 8: { fieldValue = BitConverter.ToInt64(bytes, 0); break; }
					default: { fieldValue = bytes; break; }
				}
			}
			else
			{
				switch (length)
				{
					case 1: { fieldValue = bytes[0]; break; }
					case 2: { fieldValue = BitConverter.ToUInt16(bytes, 0); break; }
					case 4: { fieldValue = BitConverter.ToUInt32(bytes, 0); break; }
					case 8: { fieldValue = BitConverter.ToUInt64(bytes, 0); break; }
					default: { fieldValue = bytes; break; }
				}
			}

			return length;
		}

		/// <summary>
		/// Reads a floating point value from the buffer.
		/// </summary>
		private int ReadField(TsCCpxContext context, FloatingPoint field, out object fieldValue)
		{
			fieldValue = null;

			var buffer = context.Buffer;

			// initialize serialization paramters.
			var length = (field.LengthSpecified) ? (int)field.Length : 4;
			var format = field.FloatFormat ?? context.FloatFormat;

			// apply defaults for built in types.
			if (field.GetType() == typeof(Single)) { length = 4; format = TsCCpxContext.FLOAT_FORMAT_IEEE754; }
			else if (field.GetType() == typeof(Double)) { length = 8; format = TsCCpxContext.FLOAT_FORMAT_IEEE754; }

			// check if there is enough data left.
			if (buffer.Length - context.Index < length)
			{
				throw new TsCCpxInvalidDataInBufferException("Unexpected end of buffer.");
			}

			// copy bytes.
			var bytes = new byte[length];

			for (var ii = 0; ii < length; ii++)
			{
				bytes[ii] = buffer[context.Index + ii];
			}

			// convert to object.
			if (format == TsCCpxContext.FLOAT_FORMAT_IEEE754)
			{
				switch (length)
				{
					case 4: { fieldValue = BitConverter.ToSingle(bytes, 0); break; }
					case 8: { fieldValue = BitConverter.ToDouble(bytes, 0); break; }
					default: { fieldValue = bytes; break; }
				}
			}
			else
			{
				fieldValue = bytes;
			}

			return length;
		}

		/// <summary>
		/// Reads a char string value from the buffer.
		/// </summary>
		private int ReadField(
			TsCCpxContext context,
			CharString field,
			int fieldIndex,
			ArrayList fieldValues,
			out object fieldValue
		)
		{
			fieldValue = null;

			var buffer = context.Buffer;

			// initialize serialization parameters.
			var charWidth = (field.CharWidthSpecified) ? (int)field.CharWidth : (int)context.CharWidth;
			var charCount = (field.LengthSpecified) ? (int)field.Length : -1;

			// apply defaults for built in types.
			if (field.GetType() == typeof(Ascii)) { charWidth = 1; }
			else if (field.GetType() == typeof(Unicode)) { charWidth = 2; }

			if (field.CharCountRef != null)
			{
				charCount = ReadReference(context, field, fieldIndex, fieldValues, field.CharCountRef);
			}

			// find null terminator
			if (charCount == -1)
			{
				charCount = 0;

				for (var ii = context.Index; ii < context.Buffer.Length - charWidth + 1; ii += charWidth)
				{
					charCount++;

					var isNull = true;

					for (var jj = 0; jj < charWidth; jj++)
					{
						if (context.Buffer[ii + jj] != 0)
						{
							isNull = false;
							break;
						}
					}

					if (isNull)
					{
						break;
					}
				}
			}

			// check if there is enough data left.
			if (buffer.Length - context.Index < charWidth * charCount)
			{
				throw new TsCCpxInvalidDataInBufferException("Unexpected end of buffer.");
			}

			if (charWidth > 2)
			{
				// copy bytes.
				var bytes = new byte[charCount * charWidth];

				for (var ii = 0; ii < charCount * charWidth; ii++)
				{
					bytes[ii] = buffer[context.Index + ii];
				}

				// swap bytes.
				if (context.BigEndian)
				{
					for (var ii = 0; ii < bytes.Length; ii += charWidth)
					{
						SwapBytes(bytes, 0, charWidth);
					}
				}

				fieldValue = bytes;
			}
			else
			{
				// copy characters.
				var chars = new char[charCount];

				for (var ii = 0; ii < charCount; ii++)
				{
					if (charWidth == 1)
					{
						chars[ii] = Convert.ToChar(buffer[context.Index + ii]);
					}
					else
					{
						var charBytes = new byte[]
                        {
                            buffer[context.Index+2*ii],
                            buffer[context.Index+2*ii+1]
                        };

						if (context.BigEndian)
						{
							SwapBytes(charBytes, 0, 2);
						}

						chars[ii] = BitConverter.ToChar(charBytes, 0);
					}
				}

				fieldValue = new string(chars).TrimEnd(new char[] { '\0' });
			}

			return charCount * charWidth;
		}

		/// <summary>
		/// Reads a bit string value from the buffer.
		/// </summary>
		private int ReadField(TsCCpxContext context, BitString field, out object fieldValue, ref byte bitOffset)
		{
			fieldValue = null;

			var buffer = context.Buffer;

			// initialize serialization paramters.
			var bits = (field.LengthSpecified) ? (int)field.Length : 8;
			var length = (bits % 8 == 0) ? bits / 8 : bits / 8 + 1;

			// check if there is enough data left.
			if (buffer.Length - context.Index < length)
			{
				throw new TsCCpxInvalidDataInBufferException("Unexpected end of buffer.");
			}

			// allocate space for the value.
			var bytes = new byte[length];

			var bitsLeft = bits;
			var mask = (byte)(~((1 << bitOffset) - 1));

			// loop until all bits read.
			for (var ii = 0; bitsLeft >= 0 && ii < length; ii++)
			{
				// add the bits from the lower byte.
				bytes[ii] = (byte)((mask & buffer[context.Index + ii]) >> bitOffset);

				// check if no more bits need to be read.
				if (bitsLeft + bitOffset <= 8)
				{
					// mask out un-needed bits.
					bytes[ii] &= (byte)((1 << bitsLeft) - 1);
					break;
				}

				// check if possible to read the next byte.
				if (context.Index + ii + 1 >= buffer.Length)
				{
					throw new TsCCpxInvalidDataInBufferException("Unexpected end of buffer.");
				}

				// add the bytes from the higher byte.
				bytes[ii] += (byte)((~mask & buffer[context.Index + ii + 1]) << (8 - bitOffset));

				// check if all done.
				if (bitsLeft <= 8)
				{
					// mask out un-needed bits.
					bytes[ii] &= (byte)((1 << bitsLeft) - 1);
					break;
				}

				// decrement the bit count.
				bitsLeft -= 8;
			}

			fieldValue = bytes;

			// update the length bit offset.
			length = (bits + bitOffset) / 8;
			bitOffset = (byte)((bits + bitOffset) % 8);

			// return the bytes read.
			return length;
		}

		/// <summary>
		/// Reads a field containing an array of values.
		/// </summary>
		private int ReadArrayField(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			ArrayList fieldValues,
			out object fieldValue
		)
		{
			fieldValue = null;

			var startIndex = context.Index;

			var array = new ArrayList();
            byte bitOffset = 0;

            object elementValue;
            // read fixed length array.
            if (field.ElementCountSpecified)
            {
                for (var ii = 0; ii < field.ElementCount; ii++)
                {
                    var bytesRead = ReadField(context, field, fieldIndex, fieldValues, out elementValue, ref bitOffset);

                    if (bytesRead == 0 && bitOffset == 0)
                    {
                        break;
                    }

                    array.Add(elementValue);

                    context.Index += bytesRead;
                }
            }

            // read variable length array.
            else if (field.ElementCountRef != null)
            {
                var count = ReadReference(context, field, fieldIndex, fieldValues, field.ElementCountRef);

                for (var ii = 0; ii < count; ii++)
                {
                    var bytesRead = ReadField(context, field, fieldIndex, fieldValues, out elementValue, ref bitOffset);

                    if (bytesRead == 0 && bitOffset == 0)
                    {
                        break;
                    }

                    array.Add(elementValue);

                    context.Index += bytesRead;
                }
            }

            // read terminated array.
            else if (field.FieldTerminator != null)
            {
                var terminator = GetTerminator(context, field);

                while (context.Index < context.Buffer.Length)
                {
                    var found = true;

                    for (var ii = 0; ii < terminator.Length; ii++)
                    {
                        if (terminator[ii] != context.Buffer[context.Index + ii])
                        {
                            found = false;
                            break;
                        }
                    }

                    if (found)
                    {
                        context.Index += terminator.Length;
                        break;
                    }

                    var bytesRead = ReadField(context, field, fieldIndex, fieldValues, out elementValue, ref bitOffset);

                    if (bytesRead == 0 && bitOffset == 0)
                    {
                        break;
                    }

                    array.Add(elementValue);

                    context.Index += bytesRead;
                }
            }

            // skip padding bits at the end of an array.
            if (bitOffset != 0)
			{
				context.Index++;
			}

			// convert array list to a fixed length array of a single type. 
			Type type = null;

			foreach (var element in array)
			{
				if (type == null)
				{
					type = element.GetType();
				}
				else
				{
					if (type != element.GetType())
					{
						type = typeof(object);
						break;
					}
				}
			}

			fieldValue = array.ToArray(type);

			// return the total bytes read.
			return (context.Index - startIndex);
		}

		/// <summary>
		/// Finds the integer value referenced by the field name.
		/// </summary>
		private int ReadReference(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			ArrayList fieldValues,
			string fieldName
		)
		{
			TsCCpxComplexValue complexValue = null;

			if (fieldName.Length == 0)
			{
				if (fieldIndex > 0 && fieldIndex - 1 < fieldValues.Count)
				{
					complexValue = (TsCCpxComplexValue)fieldValues[fieldIndex - 1];
				}
			}
			else
			{
				for (var ii = 0; ii < fieldIndex; ii++)
				{
					complexValue = (TsCCpxComplexValue)fieldValues[ii];

					if (complexValue.Name == fieldName)
					{
						break;
					}

					complexValue = null;
				}
			}

			if (complexValue == null)
			{
				throw new TsCCpxInvalidSchemaException(string.Format("Referenced field not found ({0}).", fieldName));
			}

			return Convert.ToInt32(complexValue.Value);
		}

		#endregion
	}
}
