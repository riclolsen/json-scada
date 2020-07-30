/*
 *  ApplicationLayerParameters.cs
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

namespace lib60870.CS104
{
    /// <summary>
    /// Parameters for the CS 104 APCI (Application Protocol Control Information)
    /// </summary>
    public class APCIParameters
    {
        private int k = 12;

        private int w = 8;

        private int t0 = 10;

        private int t1 = 15;

        private int t2 = 10;

        private int t3 = 20;

        public APCIParameters()
        {
        }

        public APCIParameters Clone()
        {
            APCIParameters copy = new APCIParameters();

            copy.k = k;
            copy.w = w;
            copy.t0 = t0;
            copy.t1 = t1;
            copy.t2 = t2;
            copy.t3 = t3;

            return copy;
        }

        /// <summary>
        /// number of unconfirmed APDUs in I format
        /// (range: 1 .. 32767 (2^15 - 1) - sender will
        ///  stop transmission after k unconfirmed I messages
        /// </summary>
        public int K
        {
            get
            {
                return this.k;
            }
            set
            {
                k = value;
            }
        }

        /// <summary>
        /// number of unconfirmed APDUs in I format 
        /// (range: 1 .. 32767 (2^15 - 1) - receiver
        /// will confirm latest after w messages
        /// </summary>
        public int W
        {
            get
            {
                return this.w;
            }
            set
            {
                w = value;
            }
        }

        /// <summary>
        /// Timeout for connection establishment (in s)
        /// </summary>
        /// <value>timeout t0</value>
        public int T0
        {
            get
            {
                return this.t0;
            }
            set
            {
                t0 = value;
            }
        }

        /// <summary>
        /// timeout for transmitted APDUs in I/U format (in s)
        /// when timeout elapsed without confirmation the connection
        /// will be closed
        /// </summary>
        /// <value>timeout t1</value>
        public int T1
        {
            get
            {
                return this.t1;
            }
            set
            {
                t1 = value;
            }
        }

        /// <summary>
        /// timeout to confirm messages (in s)
        /// </summary>
        /// <value>timeout t2</value>
        public int T2
        {
            get
            {
                return this.t2;
            }
            set
            {
                t2 = value;
            }
        }

        /// <summary>
        /// time until sending test telegrams in case of idle connection
        /// </summary>
        /// <value>timeout t3</value>
        public int T3
        {
            get
            {
                return this.t3;
            }
            set
            {
                t3 = value;
            }
        }
    }
}