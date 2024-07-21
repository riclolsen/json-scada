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
#endregion

namespace Technosoftware.DaAeHdaClient.Cpx
{

	/// <summary>
	/// A class that writes a complex data item to a binary buffer.
	/// </summary>
	public class TsCCpxBinaryWriter : TsCCpxBinaryStream
	{
		///////////////////////////////////////////////////////////////////////
		#region Public Methods

		/// <summary>
		/// Writes a complex value to a buffer.
		/// </summary>
		/// <param name="namedValue">The structured value to write to the buffer.</param>
		/// <param name="dictionary">The type dictionary that contains a complex type identified with the type name.</param>
		/// <param name="typeName">The name of the type that describes the data.</param>
		/// <returns>A buffer containing the binary form of the complex type.</returns>
		public byte[] Write(TsCCpxComplexValue namedValue, TypeDictionary dictionary, string typeName)
		{
			if (namedValue == null) throw new ArgumentNullException(nameof(namedValue));
			if (dictionary == null) throw new ArgumentNullException(nameof(dictionary));
			if (typeName == null) throw new ArgumentNullException(nameof(typeName));

			var context = InitializeContext(null, dictionary, typeName);

			// determine the size of buffer required.
			var bytesRequired = WriteType(context, namedValue);

			if (bytesRequired == 0)
			{
				throw new TsCCpxInvalidDataToWriteException("Could not write value into buffer.");
			}

			// allocate buffer.
			context.Buffer = new byte[bytesRequired];

			// write data into buffer.
			var bytesWritten = WriteType(context, namedValue);

			if (bytesWritten != bytesRequired)
			{
				throw new TsCCpxInvalidDataToWriteException("Could not write value into buffer.");
			}

			return context.Buffer;
		}

		#endregion

		///////////////////////////////////////////////////////////////////////
		#region Private Methods

		/// <summary>
		/// Writes an instance of a type to the buffer.
		/// </summary>
		private int WriteType(TsCCpxContext context, TsCCpxComplexValue namedValue)
		{
			var type = context.Type;
			var startIndex = context.Index;
            if (namedValue.Value == null || namedValue.Value.GetType() != typeof(TsCCpxComplexValue[]))
			{
				throw new TsCCpxInvalidDataToWriteException("Type instance does not contain field values.");
			}

            var fieldValues = (TsCCpxComplexValue[])namedValue.Value;

            if (fieldValues.Length != type.Field.Length)
			{
				throw new TsCCpxInvalidDataToWriteException("Type instance does not contain the correct number of fields.");
			}

			byte bitOffset = 0;

			for (var ii = 0; ii < type.Field.Length; ii++)
			{
				var field = type.Field[ii];
				var fieldValue = fieldValues[ii];

				if (bitOffset != 0)
				{
					if (field.GetType() != typeof(BitString))
					{
						context.Index++;
						bitOffset = 0;
					}
				}

                int bytesWritten;
                if (IsArrayField(field))
				{
					bytesWritten = WriteArrayField(context, field, ii, fieldValues, fieldValue.Value);
				}
				else if (field.GetType() == typeof(TypeReference))
				{
					bytesWritten = WriteField(context, (TypeReference)field, fieldValue);
				}
				else
				{
					bytesWritten = WriteField(context, field, ii, fieldValues, fieldValue.Value, ref bitOffset);
				}

				if (bytesWritten == 0 && bitOffset == 0)
				{
					throw new TsCCpxInvalidDataToWriteException(string.Format("Could not write field '{0}' in type '{1}'.", field.Name, type.TypeID));
				}

				context.Index += bytesWritten;
			}

			if (bitOffset != 0)
			{
				context.Index++;
			}

			return (context.Index - startIndex);
		}

		/// <summary>
		/// Writes the value contained in a field to the buffer.
		/// </summary>
		private int WriteField(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			TsCCpxComplexValue[] fieldValues,
			object fieldValue,
			ref byte bitOffset
		)
		{
			var type = field.GetType();

			if (type == typeof(Integer) || type.IsSubclassOf(typeof(Integer)))
			{
				return WriteField(context, (Integer)field, fieldValue);
			}
			else if (type == typeof(FloatingPoint) || type.IsSubclassOf(typeof(FloatingPoint)))
			{
				return WriteField(context, (FloatingPoint)field, fieldValue);
			}
			else if (type == typeof(CharString) || type.IsSubclassOf(typeof(CharString)))
			{
				return WriteField(context, (CharString)field, fieldIndex, fieldValues, fieldValue);
			}
			else if (type == typeof(BitString) || type.IsSubclassOf(typeof(BitString)))
			{
				return WriteField(context, (BitString)field, fieldValue, ref bitOffset);
			}
			else if (type == typeof(TypeReference) || type.IsSubclassOf(typeof(TypeReference)))
			{
				return WriteField(context, (TypeReference)field, fieldValue);
			}
			else
			{
				throw new NotImplementedException(string.Format("Fields of type '{0}' are not implemented yet.", type));
			}
		}

		/// <summary>
		/// Writes a complex type from to the buffer.
		/// </summary>
		private int WriteField(TsCCpxContext context, TypeReference field, object fieldValue)
		{
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

			if (fieldValue.GetType() != typeof(TsCCpxComplexValue))
			{
				throw new TsCCpxInvalidDataToWriteException("Instance of type is not the correct type.");
			}

			return WriteType(context, (TsCCpxComplexValue)fieldValue);
		}

		/// <summary>
		/// Writes a integer value from to the buffer.
		/// </summary>
		private int WriteField(TsCCpxContext context, Integer field, object fieldValue)
		{
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

			// only write to the buffer if it has been allocated.
			if (buffer != null)
			{
				// check if there is enough data left.
				if (buffer.Length - context.Index < length)
				{
					throw new TsCCpxInvalidDataToWriteException("Unexpected end of buffer.");
				}

                // copy and swap bytes if required.
                byte[] bytes;
                if (signed)
				{
					switch (length)
					{
						case 1:
							{
								bytes = new byte[1];

								var value = Convert.ToSByte(fieldValue);

								if (value < 0)
								{
									bytes[0] = (byte)(byte.MaxValue + value + 1);
								}
								else
								{
									bytes[0] = (byte)value;
								}

								break;
							}

						case 2: { bytes = BitConverter.GetBytes(Convert.ToInt16(fieldValue)); break; }
						case 4: { bytes = BitConverter.GetBytes(Convert.ToInt32(fieldValue)); break; }
						case 8: { bytes = BitConverter.GetBytes(Convert.ToInt64(fieldValue)); break; }
						default: { bytes = (byte[])fieldValue; break; }
					}
				}
				else
				{
					switch (length)
					{
						case 1: { bytes = new byte[] { Convert.ToByte(fieldValue) }; break; }
						case 2: { bytes = BitConverter.GetBytes(Convert.ToUInt16(fieldValue)); break; }
						case 4: { bytes = BitConverter.GetBytes(Convert.ToUInt32(fieldValue)); break; }
						case 8: { bytes = BitConverter.GetBytes(Convert.ToUInt64(fieldValue)); break; }
						default: { bytes = (byte[])fieldValue; break; }
					}
				}

				// copy and swap bytes.
				if (context.BigEndian)
				{
					SwapBytes(bytes, 0, length);
				}

				// write bytes to buffer.
				for (var ii = 0; ii < bytes.Length; ii++)
				{
					buffer[context.Index + ii] = bytes[ii];
				}
			}

			return length;
		}

		/// <summary>
		/// Writes a integer value from to the buffer.
		/// </summary>
		private int WriteField(TsCCpxContext context, FloatingPoint field, object fieldValue)
		{
			var buffer = context.Buffer;

			// initialize serialization paramters.
			var length = (field.LengthSpecified) ? (int)field.Length : 4;
			var format = field.FloatFormat ?? context.FloatFormat;

			// apply defaults for built in types.
			if (field.GetType() == typeof(Single)) { length = 4; format = TsCCpxContext.FLOAT_FORMAT_IEEE754; }
			else if (field.GetType() == typeof(Double)) { length = 8; format = TsCCpxContext.FLOAT_FORMAT_IEEE754; }

			// only write to the buffer if it has been allocated.
			if (buffer != null)
			{
				// check if there is enough data left.
				if (buffer.Length - context.Index < length)
				{
					throw new TsCCpxInvalidDataToWriteException("Unexpected end of buffer.");
				}

                // copy bytes if required.
                byte[] bytes;
                if (format == TsCCpxContext.FLOAT_FORMAT_IEEE754)
				{
					switch (length)
					{
						case 4: { bytes = BitConverter.GetBytes(Convert.ToSingle(fieldValue)); break; }
						case 8: { bytes = BitConverter.GetBytes(Convert.ToDouble(fieldValue)); break; }
						default: { bytes = (byte[])fieldValue; break; }
					}
				}
				else
				{
					bytes = (byte[])fieldValue;
				}

				// write bytes to buffer.
				for (var ii = 0; ii < bytes.Length; ii++)
				{
					buffer[context.Index + ii] = bytes[ii];
				}
			}

			return length;
		}

		/// <summary>
		/// Writes a char string value to the buffer.
		/// </summary>
		private int WriteField(
			TsCCpxContext context,
			CharString field,
			int fieldIndex,
			TsCCpxComplexValue[] fieldValues,
			object fieldValue
		)
		{
			var buffer = context.Buffer;

			// initialize serialization parameters.
			var charWidth = (field.CharWidthSpecified) ? (int)field.CharWidth : (int)context.CharWidth;
			var charCount = (field.LengthSpecified) ? (int)field.Length : -1;

			// apply defaults for built in types.
			if (field.GetType() == typeof(Ascii)) { charWidth = 1; }
			else if (field.GetType() == typeof(Unicode)) { charWidth = 2; }

			byte[] bytes = null;

			if (charCount == -1)
			{
				// extra wide characters stored as byte arrays
				if (charWidth > 2)
				{
					if (fieldValue.GetType() != typeof(byte[]))
					{
						throw new TsCCpxInvalidDataToWriteException("Field value is not a byte array.");
					}

					bytes = (byte[])fieldValue;
					charCount = bytes.Length / charWidth;
				}

				// convert string to byte array.
				else
				{
					if (fieldValue.GetType() != typeof(string))
					{
						throw new TsCCpxInvalidDataToWriteException("Field value is not a string.");
					}

					var stringValue = (string)fieldValue;

					charCount = stringValue.Length + 1;

					// calculate length of ascii string by forcing pure unicode characters to two ascii chars.
					if (charWidth == 1)
					{
						charCount = 1;

						foreach (var unicodeChar in stringValue)
						{
							charCount++;

							var charBytes = BitConverter.GetBytes(unicodeChar);

							if (charBytes[1] != 0)
							{
								charCount++;
							}
						}
					}
				}
			}

			// update the char count reference.
			if (field.CharCountRef != null)
			{
				WriteReference(context, field, fieldIndex, fieldValues, field.CharCountRef, charCount);
			}

			if (buffer != null)
			{
				// copy string to buffer.
				if (bytes == null)
				{
					var stringValue = (string)fieldValue;

					bytes = new byte[charWidth * charCount];

					var index = 0;

					for (var ii = 0; ii < stringValue.Length; ii++)
					{
						if (index >= bytes.Length)
						{
							break;
						}

						var charBytes = BitConverter.GetBytes(stringValue[ii]);

						bytes[index++] = charBytes[0];

						if (charWidth == 2 || charBytes[1] != 0)
						{
							bytes[index++] = charBytes[1];
						}
					}
				}

				// check if there is enough data left.
				if (buffer.Length - context.Index < bytes.Length)
				{
					throw new TsCCpxInvalidDataToWriteException("Unexpected end of buffer.");
				}

				// write bytes to buffer.
				for (var ii = 0; ii < bytes.Length; ii++)
				{
					buffer[context.Index + ii] = bytes[ii];
				}

				// swap bytes.
				if (context.BigEndian && charWidth > 1)
				{
					for (var ii = 0; ii < bytes.Length; ii += charWidth)
					{
						SwapBytes(buffer, context.Index + ii, charWidth);
					}
				}
			}

			return charCount * charWidth;
		}

		/// <summary>
		/// Writes a bit string value to the buffer.
		/// </summary>
		private int WriteField(TsCCpxContext context, BitString field, object fieldValue, ref byte bitOffset)
		{
			var buffer = context.Buffer;

			// initialize serialization paramters.
			var bits = (field.LengthSpecified) ? (int)field.Length : 8;
			var length = (bits % 8 == 0) ? bits / 8 : bits / 8 + 1;

			if (fieldValue.GetType() != typeof(byte[]))
			{
				throw new TsCCpxInvalidDataToWriteException("Wrong data type to write.");
			}

			// allocate space for the value.
			var bytes = (byte[])fieldValue;

			if (buffer != null)
			{
				// check if there is enough data left.
				if (buffer.Length - context.Index < length)
				{
					throw new TsCCpxInvalidDataToWriteException("Unexpected end of buffer.");
				}

				var bitsLeft = bits;
				var mask = (bitOffset == 0) ? (byte)0xFF : (byte)((0x80 >> (bitOffset - 1)) - 1);

				// loop until all bits read.
				for (var ii = 0; bitsLeft >= 0 && ii < length; ii++)
				{
					// add the bits from the lower byte.
					buffer[context.Index + ii] += (byte)((mask & ((1 << bitsLeft) - 1) & bytes[ii]) << bitOffset);

					// check if no more bits need to be read.
					if (bitsLeft + bitOffset <= 8)
					{
						break;
					}

					// check if possible to read the next byte.
					if (context.Index + ii + 1 >= buffer.Length)
					{
						throw new TsCCpxInvalidDataToWriteException("Unexpected end of buffer.");
					}

					// add the bytes from the higher byte.
					buffer[context.Index + ii + 1] += (byte)((~mask & ((1 << bitsLeft) - 1) & bytes[ii]) >> (8 - bitOffset));

					// check if all done.
					if (bitsLeft <= 8)
					{
						break;
					}

					// decrement the bit count.
					bitsLeft -= 8;
				}
			}

			// update the length bit offset.
			length = (bits + bitOffset) / 8;
			bitOffset = (byte)((bits + bitOffset) % 8);

			// return the bytes read.
			return length;
		}

		/// <summary>
		/// Reads a field containing an array of values.
		/// </summary>
		private int WriteArrayField(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			TsCCpxComplexValue[] fieldValues,
			object fieldValue
		)
		{
			var startIndex = context.Index;
            if (!fieldValue.GetType().IsArray)
			{
				throw new TsCCpxInvalidDataToWriteException("Array field value is not an array type.");
			}

            var array = (Array)fieldValue;

            byte bitOffset = 0;

			// read fixed length array.
			if (field.ElementCountSpecified)
			{
				var count = 0;

				foreach (var elementValue in array)
				{
					// ignore any excess elements.
					if (count == field.ElementCount)
					{
						break;
					}

					var bytesWritten = WriteField(context, field, fieldIndex, fieldValues, elementValue, ref bitOffset);

					if (bytesWritten == 0 && bitOffset == 0)
					{
						break;
					}

					context.Index += bytesWritten;
					count++;
				}

				// write a null value for any missing elements.
				while (count < field.ElementCount)
				{
					var bytesWritten = WriteField(context, field, fieldIndex, fieldValues, null, ref bitOffset);

					if (bytesWritten == 0 && bitOffset == 0)
					{
						break;
					}

					context.Index += bytesWritten;
					count++;
				}
			}

			// read variable length array.
			else if (field.ElementCountRef != null)
			{
				var count = 0;

				foreach (var elementValue in array)
				{
					var bytesWritten = WriteField(context, field, fieldIndex, fieldValues, elementValue, ref bitOffset);

					if (bytesWritten == 0 && bitOffset == 0)
					{
						break;
					}

					context.Index += bytesWritten;
					count++;
				}

				// update the value of the referenced field with the correct element count.
				WriteReference(context, field, fieldIndex, fieldValues, field.ElementCountRef, count);
			}

			// read terminated array.
			else if (field.FieldTerminator != null)
			{
				foreach (var elementValue in array)
				{
					var bytesWritten = WriteField(context, field, fieldIndex, fieldValues, elementValue, ref bitOffset);

					if (bytesWritten == 0 && bitOffset == 0)
					{
						break;
					}

					context.Index += bytesWritten;
				}

				// get the terminator.
				var terminator = GetTerminator(context, field);

				if (context.Buffer != null)
				{
					// write the terminator.
					for (var ii = 0; ii < terminator.Length; ii++)
					{
						context.Buffer[context.Index + ii] = terminator[ii];
					}
				}

				context.Index += terminator.Length;
			}

			// clear the bit offset and skip to end of byte at the end of the array.
			if (bitOffset != 0)
			{
				context.Index++;
			}

			// return the total bytes read.
			return (context.Index - startIndex);
		}

		/// <summary>
		/// Finds the integer value referenced by the field name.
		/// </summary>
		private void WriteReference(
			TsCCpxContext context,
			FieldType field,
			int fieldIndex,
			TsCCpxComplexValue[] fieldValues,
			string fieldName,
			int count
		)
		{
			TsCCpxComplexValue namedValue = null;

			if (fieldName.Length == 0)
			{
				if (fieldIndex > 0 && fieldIndex - 1 < fieldValues.Length)
				{
					namedValue = (TsCCpxComplexValue)fieldValues[fieldIndex - 1];
				}
			}
			else
			{
				for (var ii = 0; ii < fieldIndex; ii++)
				{
					namedValue = (TsCCpxComplexValue)fieldValues[ii];

					if (namedValue.Name == fieldName)
					{
						break;
					}

					namedValue = null;
				}
			}

			if (namedValue == null)
			{
				throw new TsCCpxInvalidSchemaException(string.Format("Referenced field not found ({0}).", fieldName));
			}

			if (context.Buffer == null)
			{
				namedValue.Value = count;
			}

			if (!count.Equals(namedValue.Value))
			{
				throw new TsCCpxInvalidDataToWriteException("Reference field value and the actual array length are not equal.");
			}
		}

		#endregion
	}
}
