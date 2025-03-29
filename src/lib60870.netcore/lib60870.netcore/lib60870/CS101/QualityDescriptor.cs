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

namespace lib60870.CS101
{
    public class QualityDescriptor
    {
        private byte encodedValue;

        public static QualityDescriptor VALID()
        {
            return new QualityDescriptor();
        }

        public static QualityDescriptor INVALID()
        {
            var qd = new QualityDescriptor();
            qd.Invalid = true;
            return qd;
        }

        public QualityDescriptor()
        {
            encodedValue = 0;
        }

        public QualityDescriptor(QualityDescriptor original)
        {
            encodedValue = original.encodedValue;
        }

        public QualityDescriptor(byte encodedValue)
        {
            this.encodedValue = encodedValue;
        }

        public override bool Equals(object obj)
        {
            if (obj == null)
                return false;

            if (!(obj is QualityDescriptor))
                return false;

            return (encodedValue == ((QualityDescriptor)obj).encodedValue);
        }

        public override int GetHashCode()
        {
            return encodedValue.GetHashCode();
        }

        public bool Overflow
        {
            get
            {
                if ((encodedValue & 0x01) != 0)
                    return true;
                else
                    return false;
            }

            set
            {
                if (value)
                    encodedValue |= 0x01;
                else
                    encodedValue &= 0xfe;
            }
        }

        public bool Blocked
        {
            get
            {
                if ((encodedValue & 0x10) != 0)
                    return true;
                else
                    return false;
            }

            set
            {
                if (value)
                    encodedValue |= 0x10;
                else
                    encodedValue &= 0xef;
            }
        }

        public bool Substituted
        {
            get
            {
                if ((encodedValue & 0x20) != 0)
                    return true;
                else
                    return false;
            }

            set
            {
                if (value)
                    encodedValue |= 0x20;
                else
                    encodedValue &= 0xdf;
            }
        }


        public bool NonTopical
        {
            get
            {
                if ((encodedValue & 0x40) != 0)
                    return true;
                else
                    return false;
            }

            set
            {
                if (value)
                    encodedValue |= 0x40;
                else
                    encodedValue &= 0xbf;
            }
        }


        public bool Invalid
        {
            get
            {
                if ((encodedValue & 0x80) != 0)
                    return true;
                else
                    return false;
            }

            set
            {
                if (value)
                    encodedValue |= 0x80;
                else
                    encodedValue &= 0x7f;
            }
        }

        public byte EncodedValue
        {
            get
            {
                return encodedValue;
            }
            set
            {
                encodedValue = value;
            }
        }

        public override string ToString()
        {
            return string.Format("[QualityDescriptor: Overflow={0}, Blocked={1}, Substituted={2}, NonTopical={3}, Invalid={4}]", Overflow, Blocked, Substituted, NonTopical, Invalid);
        }
    }

}

