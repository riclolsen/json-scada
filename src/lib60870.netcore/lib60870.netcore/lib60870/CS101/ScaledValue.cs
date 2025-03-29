/*
 *  Copyright 2016-2025 Michael Zillgith
 *
 *  This file is part of lib60870.NET
 *
 *  lib60870.NET is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  lib60870.NET is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with lib60870.NET.  If not, see <http://www.gnu.org/licenses/>.
 *
 *  See COPYING file for the complete license text.
 */

using System;

namespace lib60870.CS101
{
    public class ScaledValue
    {
        private byte[] encodedValue = new byte[2];
        const int SCALED_VALUE_MAX = 32767;
        const int SCALED_VALUE_MIN = -32768;
        const float NORMALIZED_VALUE_MAX = 32767f / 32768f;
        const float NORMALIZED_VALUE_MIN = -1.0f;

        public ScaledValue(byte[] msg, int startIndex)
        {
            if (msg.Length < startIndex + 2)
                throw new ASDUParsingException("Message too small for parsing ScaledValue");

            for (int i = 0; i < 2; i++)
                encodedValue[i] = msg[startIndex + i];
        }

        public ScaledValue()
        {
        }

        public ScaledValue(int value)
        {
            Value = value;
        }

        public ScaledValue(short value)
        {
            ShortValue = value;
        }

        public ScaledValue(ScaledValue original)
        {
            ShortValue = original.ShortValue;
        }

        public byte[] GetEncodedValue()
        {
            return encodedValue;
        }

        public int Value
        {
            get
            {
                int value;

                value = encodedValue[0] | (encodedValue[1] << 8);

                /* Sign-extend for negative values*/
                if (value > 32767)
                    value -= 65536;

                /* Clamp within valid range*/
                if (value > SCALED_VALUE_MAX)
                    value = SCALED_VALUE_MAX;
                else if (value < SCALED_VALUE_MIN)
                    value = SCALED_VALUE_MIN;

               
                return value;
            }
            set
            {                if (value > SCALED_VALUE_MAX)
                    value = SCALED_VALUE_MAX;
                else if (value < SCALED_VALUE_MIN)
                    value = SCALED_VALUE_MIN;

                encodedValue[0] = (byte)(value & 0xFF);   /* Lower byte*/
                encodedValue[1] = (byte)((value >> 8) & 0xFF); /* Upper byte*/
            }
        }

        public short ShortValue
        {
            get
            {
                UInt16 uintVal;

                uintVal = encodedValue[0];
                uintVal += (UInt16)(encodedValue[1] * 0x100);

                return (short)uintVal;
            }

            set
            {
                UInt16 uintVal = (UInt16)value;

                encodedValue[0] = (byte)(uintVal % 256);
                encodedValue[1] = (byte)(uintVal / 256);
            }
        }

        public override string ToString()
        {
            return "" + Value;
        }

        public float GetNormalizedValue()
        {
            /* Ensure correct floating-point division*/
            double result = Value / 32767.0;

            /* Ensure proper clamping*/
            if (result > NORMALIZED_VALUE_MAX)
                result = NORMALIZED_VALUE_MAX;
            if (result < -1.0)
                result = -1.0;

            return (float)result;
        }

        public int ConvertNormalizedValueToScaled(float value)
        {

            if (value > NORMALIZED_VALUE_MAX)

                value = NORMALIZED_VALUE_MAX;

            else if (value < NORMALIZED_VALUE_MIN)

                value = NORMALIZED_VALUE_MIN;

            float scaledValue = value * 32768f;

            return (int)(scaledValue < 0 ? scaledValue - 0.5f : scaledValue + 0.5f);

        }

        public void SetScaledFromNormalizedValue(float value)
        {

            if (value > NORMALIZED_VALUE_MAX)

                value = NORMALIZED_VALUE_MAX;

            else if (value < NORMALIZED_VALUE_MIN)

                value = NORMALIZED_VALUE_MIN;

            float scaledValue = value * 32768f;

            Value = (int)(scaledValue < 0 ? scaledValue - 0.5f : scaledValue + 0.5f);

        }
    }

}
