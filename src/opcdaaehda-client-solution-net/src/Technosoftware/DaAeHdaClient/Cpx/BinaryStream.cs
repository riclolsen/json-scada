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
	/// A class that reads a complex data item from a binary buffer.
	/// </summary>
	public class TsCCpxBinaryStream
	{
		///////////////////////////////////////////////////////////////////////
		#region Constructors, Destructor, Initialization

		/// <summary>
		/// Initializes the binary stream with defaults.
		/// </summary>
		protected TsCCpxBinaryStream() { }

		#endregion

		///////////////////////////////////////////////////////////////////////
		#region Internal Methods

		/// <summary>
		/// Determines if a field contains an array of values.
		/// </summary>
		internal bool IsArrayField(FieldType field)
		{
			if (field.ElementCountSpecified)
			{
				if (field.ElementCountRef != null || field.FieldTerminator != null)
				{
					throw new TsCCpxInvalidSchemaException(string.Format("Multiple array size attributes specified for field '{0} '.", field.Name));
				}

				return true;
			}

			if (field.ElementCountRef != null)
			{
				if (field.FieldTerminator != null)
				{
					throw new TsCCpxInvalidSchemaException(string.Format("Multiple array size attributes specified for field '{0} '.", field.Name));
				}

				return true;
			}

			if (field.FieldTerminator != null)
			{
				return true;
			}

			return false;
		}

		/// <summary>
		/// Returns the termininator for the field.
		/// </summary>
		internal byte[] GetTerminator(TsCCpxContext context, FieldType field)
		{
			if (field.FieldTerminator == null) throw new TsCCpxInvalidSchemaException(string.Format("{0} is not a terminated subscription.", field.Name));

			var terminator = Convert.ToString(field.FieldTerminator).ToUpper();

			var bytes = new byte[terminator.Length / 2];

			for (var ii = 0; ii < bytes.Length; ii++)
			{
				bytes[ii] = Convert.ToByte(terminator.Substring(ii * 2, 2), 16);
			}

			return bytes;
		}

		/// <summary>
		/// Looks up the type name in the dictionary and initializes the context.
		/// </summary>
		internal TsCCpxContext InitializeContext(byte[] buffer, TypeDictionary dictionary, string typeName)
		{
			var context = new TsCCpxContext(buffer) { Dictionary = dictionary, Type = null, BigEndian = dictionary.DefaultBigEndian, CharWidth = dictionary.DefaultCharWidth, StringEncoding = dictionary.DefaultStringEncoding, FloatFormat = dictionary.DefaultFloatFormat };

			foreach (var type in dictionary.TypeDescription)
			{
				if (type.TypeID == typeName)
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
				throw new TsCCpxInvalidSchemaException(string.Format("Type '{0}' not found in dictionary.", typeName));
			}

			return context;
		}

		/// <summary>
		/// Swaps the order of bytes in the buffer.
		/// </summary>
		internal void SwapBytes(byte[] bytes, int index, int length)
		{
			for (var ii = 0; ii < length / 2; ii++)
			{
				var temp = bytes[index + length - 1 - ii];
				bytes[index + length - 1 - ii] = bytes[index + ii];
				bytes[index + ii] = temp;
			}
		}

		#endregion
	}
}
