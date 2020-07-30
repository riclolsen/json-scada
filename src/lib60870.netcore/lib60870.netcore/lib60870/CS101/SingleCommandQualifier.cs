/*
 *  Copyright 2016 MZ Automation GmbH
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

namespace lib60870
{
    public class SingleCommandQualifier
    {
        private byte encodedValue;

        public SingleCommandQualifier(byte encodedValue)
        {
            this.encodedValue = encodedValue;
        }

        public SingleCommandQualifier(bool state, bool selectCommand, int qu)
        {
            encodedValue = (byte)((qu & 0x1f) * 4);

            if (state)
                encodedValue |= 0x01;

            if (selectCommand)
                encodedValue |= 0x80;
        }

        public int QU
        {
            get
            {
                return ((encodedValue & 0x7c) / 4);
            }
        }

        public bool State
        {
            get
            {
                return ((encodedValue & 0x01) == 0x01);
            }
        }

        public bool Select
        {
            get
            {
                return ((encodedValue & 0x80) == 0x80);
            }
        }

        public byte GetEncodedValue()
        {
            return encodedValue;
        }

    }
}

