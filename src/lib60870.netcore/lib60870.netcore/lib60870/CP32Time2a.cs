/*
 *  CP32Time2a.cs
 *
 *  Copyright 2017 MZ Automation GmbH
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

    public class CP32Time2a
    {
        private byte[] encodedValue = new byte[4];

        internal CP32Time2a(byte[] msg, int startIndex)
        {
            if (msg.Length < startIndex + 4)
                throw new ASDUParsingException("Message too small for parsing CP56Time2a");

            for (int i = 0; i < 4; i++)
                encodedValue[i] = msg[startIndex + i];
        }

        public CP32Time2a(int hours, int minutes, int seconds, int milliseconds, bool invalid, bool summertime)
        {
            Hour = hours;
            Minute = minutes;
            Second = seconds;
            Millisecond = milliseconds;
            Invalid = invalid;
            SummerTime = summertime;
        }

        public CP32Time2a(DateTime time)
        {
            Millisecond = time.Millisecond;
            Second = time.Second;
            Hour = time.Hour;
            Minute = time.Minute;
        }

        public CP32Time2a()
        {
            for (int i = 0; i < 4; i++)
                encodedValue[i] = 0;
        }

        /// <summary>
        /// Gets the date time added to the reference day.
        /// </summary>
        /// <returns>The date time.</returns>
        /// <param name="refTime">Datetime representing the reference day</param>
        public DateTime GetDateTime(DateTime refTime)
        {
            DateTime newTime = new DateTime(refTime.Year, refTime.Month, refTime.Day, Hour, Minute, Second, Millisecond);

            return newTime;
        }

        public DateTime GetDateTime()
        {
            return GetDateTime(DateTime.Now);
        }


        /// <summary>
        /// Gets or sets the millisecond part of the time value (range 0 to 999)
        /// </summary>
        /// <value>The millisecond.</value>
        public int Millisecond
        {
            get
            {
                return (encodedValue[0] + (encodedValue[1] * 0x100)) % 1000;
            }

            set
            {
                int millies = (Second * 1000) + value;

                encodedValue[0] = (byte)(millies & 0xff);
                encodedValue[1] = (byte)((millies / 0x100) & 0xff);
            }
        }

        /// <summary>
        /// Gets or sets the second (range 0 to 59)
        /// </summary>
        /// <value>The second.</value>
        public int Second
        {
            get
            {
                return  (encodedValue[0] + (encodedValue[1] * 0x100)) / 1000;
            }

            set
            {
                int millies = encodedValue[0] + (encodedValue[1] * 0x100);

                int msPart = millies % 1000;

                millies = (value * 1000) + msPart;

                encodedValue[0] = (byte)(millies & 0xff);
                encodedValue[1] = (byte)((millies / 0x100) & 0xff);
            }
        }

        /// <summary>
        /// Gets or sets the minute (range 0 to 59)
        /// </summary>
        /// <value>The minute.</value>
        public int Minute
        {
            get
            {
                return (encodedValue[2] & 0x3f);
            }

            set
            {
                encodedValue[2] = (byte)((encodedValue[2] & 0xc0) | (value & 0x3f));
            }
        }

        /// <summary>
        /// Gets or sets the hour (range 0 to 23)
        /// </summary>
        /// <value>The hour.</value>
        public int Hour
        {
            get
            {
                return (encodedValue[3] & 0x1f);
            }

            set
            {
                encodedValue[3] = (byte)((encodedValue[3] & 0xe0) | (value & 0x1f));
            }
        }

        public bool SummerTime
        {
            get
            {
                return ((encodedValue[3] & 0x80) != 0);
            }

            set
            {
                if (value)
                    encodedValue[3] |= 0x80;
                else
                    encodedValue[3] &= 0x7f;
            }
        }

        /// <summary>
        /// Gets a value indicating whether this <see cref="lib60870.CP56Time2a"/> is invalid.
        /// </summary>
        /// <value><c>true</c> if invalid; otherwise, <c>false</c>.</value>
        public bool Invalid
        {
            get
            {
                return ((encodedValue[2] & 0x80) != 0);
            }

            set
            {
                if (value)
                    encodedValue[2] |= 0x80;
                else
                    encodedValue[2] &= 0x7f;
            }
        }

        /// <summary>
        /// Gets a value indicating whether this <see cref="lib60870.CP26Time2a"/> was substitued by an intermediate station
        /// </summary>
        /// <value><c>true</c> if substitued; otherwise, <c>false</c>.</value>
        public bool Substituted
        {
            get
            {
                return ((encodedValue[2] & 0x40) == 0x40);
            }

            set
            {
                if (value)
                    encodedValue[2] |= 0x40;
                else
                    encodedValue[2] &= 0xbf;
            }
        }

        public byte[] GetEncodedValue()
        {
            return encodedValue;
        }

        public override string ToString()
        {
            return string.Format("[CP32Time2a: Millisecond={0}, Second={1}, Minute={2}, Hour={3}, SummerTime={4}, Invalid={5} Substituted={6}]", Millisecond, Second, Minute, Hour, SummerTime, Invalid, Substituted);
        }

    }

}

