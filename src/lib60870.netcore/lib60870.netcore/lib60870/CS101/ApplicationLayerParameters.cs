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

namespace lib60870.CS101
{
    public class ApplicationLayerParameters
    {
        public static int IEC60870_5_104_MAX_ASDU_LENGTH = 249;

        private int sizeOfTypeId = 1;

        /* VSQ = variable sturcture qualifier */
        private int sizeOfVSQ = 1;

        /* (parameter b) COT = cause of transmission (1/2) */
        private int sizeOfCOT = 2;

        private int originatorAddress = 0;

        /* (parameter a) CA = common address of ASDUs (1/2) */
        private int sizeOfCA = 2;

        /* (parameter c) IOA = information object address (1/2/3) */
        private int sizeOfIOA = 3;

        /* maximum length of ASDU */
        private int maxAsduLength = IEC60870_5_104_MAX_ASDU_LENGTH;

        public ApplicationLayerParameters()
        {
        }

        public ApplicationLayerParameters Clone()
        {
            ApplicationLayerParameters copy = new ApplicationLayerParameters();

            copy.sizeOfTypeId = sizeOfTypeId;
            copy.sizeOfVSQ = sizeOfVSQ;
            copy.sizeOfCOT = sizeOfCOT;
            copy.originatorAddress = originatorAddress;
            copy.sizeOfCA = sizeOfCA;
            copy.sizeOfIOA = sizeOfIOA;
            copy.maxAsduLength = maxAsduLength;

            return copy;
        }

        public int SizeOfCOT
        {
            get
            {
                return this.sizeOfCOT;
            }
            set
            {
                sizeOfCOT = value;
            }
        }

        public int OA
        {
            get
            {
                return this.originatorAddress;
            }
            set
            {
                originatorAddress = value;
            }
        }

        public int SizeOfCA
        {
            get
            {
                return this.sizeOfCA;
            }
            set
            {
                sizeOfCA = value;
            }
        }

        public int SizeOfIOA
        {
            get
            {
                return this.sizeOfIOA;
            }
            set
            {
                sizeOfIOA = value;
            }
        }


        public int SizeOfTypeId
        {
            get
            {
                return this.sizeOfTypeId;
            }
        }

        public int SizeOfVSQ
        {
            get
            {
                return this.sizeOfVSQ;
            }
        }

        public int MaxAsduLength
        {
            get
            {
                return this.maxAsduLength;
            }
            set
            {
                maxAsduLength = value;
            }
        }
    }
}

