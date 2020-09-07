using System;
using System.Collections.Generic;
using System.Text;

namespace libplctag.DataTypes
{
    public class StringPlcMapper : PlcMapperBase<string>, IPlcMapper<string>, IPlcMapper<string[]>
    {

        const int MAX_CONTROLLOGIX_STRING_LENGTH = 82;
        const int MAX_LOGIXPCCC_STRING_LENGTH = 82;

        override public int? ElementSize
        {
            get
            {
                switch (PlcType)
                {
                    case PlcType.ControlLogix: return 88;
                    case PlcType.Plc5: return 84;
                    case PlcType.Slc500: return 84;
                    case PlcType.LogixPccc: return 84;
                    case PlcType.Micro800: return 256; //To be Confirmed
                    case PlcType.MicroLogix: return 84;
                    default: throw new NotImplementedException();
                }
            }
        }


        override public string Decode(Tag tag, int offset)
        {
            switch (PlcType)
            {
                case PlcType.Plc5:
                case PlcType.Slc500:
                case PlcType.LogixPccc:
                case PlcType.MicroLogix:
                    return LogixPcccDecode(tag, offset);
                case PlcType.Micro800:
                    return Micro800Decode(tag, offset);
                case PlcType.ControlLogix:
                    return ControlLogixDecode(tag, offset);
                default: throw new NotImplementedException();
            }
        }

        override public void Encode(Tag tag, int offset, string value)
        {
            switch (PlcType)
            {
                case PlcType.Plc5:
                case PlcType.Slc500:
                case PlcType.LogixPccc:
                case PlcType.MicroLogix:
                    LogixPcccEncode(tag, offset, value); break;
                case PlcType.Micro800:
                    Micro800Encode(tag, offset, value); break;
                case PlcType.ControlLogix:
                    ControlLogixEncode(tag, offset, value); break;
                default: break;
            }
        }



        string ControlLogixDecode(Tag tag, int offset)
        {
            var apparentStringLength = tag.GetInt32(offset);

            var actualStringLength = Math.Min(apparentStringLength, MAX_CONTROLLOGIX_STRING_LENGTH);

            var asciiEncodedString = new byte[actualStringLength];
            for (int ii = 0; ii < actualStringLength; ii++)
            {
                asciiEncodedString[ii] = tag.GetUInt8(offset + 4 + 2 + ii);
            }

            return Encoding.ASCII.GetString(asciiEncodedString);
        }

        void ControlLogixEncode(Tag tag, int offset, string value)
        {
            if (value.Length > MAX_CONTROLLOGIX_STRING_LENGTH)
                throw new ArgumentException("String length exceeds maximum for a tag of type STRING");

            var asciiEncodedString = Encoding.ASCII.GetBytes(value);

            tag.SetInt16(offset, Convert.ToInt16(value.Length));

            for (int ii = 0; ii < asciiEncodedString.Length; ii++)
            {
                tag.SetUInt8(offset + 4 + 2 + ii, Convert.ToByte(asciiEncodedString[ii]));
            }
        }


        string Micro800Decode(Tag tag, int offset)
        {
            throw new NotImplementedException();
        }

        void Micro800Encode(Tag tag, int offset, string value)
        {
            throw new NotImplementedException();
        }


        string LogixPcccDecode(Tag tag, int offset)
        {
            var apparentStringLength = (int)tag.GetInt16(offset);

            var actualStringLength = Math.Min(apparentStringLength, MAX_LOGIXPCCC_STRING_LENGTH);

            var readLength = actualStringLength + (actualStringLength % 2); //read 1 more if odd number

            var asciiEncodedString = new byte[actualStringLength];

            for (int ii = 0; ii < readLength - 1; ii += 2)
            {
                asciiEncodedString[ii] = tag.GetUInt8(offset + 2 + ii + 1);
                if ((apparentStringLength % 2 == 0) || (ii < (actualStringLength - 1)))
                //don't do for last char in odd number string (i.e. don't get the /0)
                {
                    asciiEncodedString[ii + 1] = tag.GetUInt8(offset + 2 + ii);
                }
            }

            return Encoding.ASCII.GetString(asciiEncodedString);
        }


        void LogixPcccEncode(Tag tag, int offset, string value)
        {
            if (value.Length > MAX_LOGIXPCCC_STRING_LENGTH)
                throw new ArgumentException("String length exceeds maximum for a tag of type STRING");

            var writeLength = value.Length + (value.Length % 2); //add 1 to write length if odd

            var asciiEncodedString = Encoding.ASCII.GetBytes(value);

            tag.SetInt16(offset, Convert.ToInt16(value.Length));

            for (int ii = 0; ii < (writeLength - 1); ii += 2)
            {
                if ((value.Length % 2 == 0) || (ii < (writeLength - 2)))
                //if odd number string then set penultimate char (1 after string end) as /00
                {
                    tag.SetUInt8(offset + 2 + ii, Convert.ToByte(asciiEncodedString[ii + 1]));
                }
                else
                {
                    tag.SetUInt8(offset + 2 + ii, 0x00);
                }
                tag.SetUInt8(offset + 2 + ii + 1, Convert.ToByte(asciiEncodedString[ii]));
            }
        }

    }
}