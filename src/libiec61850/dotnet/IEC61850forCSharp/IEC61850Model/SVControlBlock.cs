/*
 *  IEC61850ServerAPI.cs
 *
 *  Copyright 2016-2025 Michael Zillgith
 *
 *  This file is part of libIEC61850.
 *
 *  libIEC61850 is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  libIEC61850 is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with libIEC61850.  If not, see <http://www.gnu.org/licenses/>.
 *
 *  See COPYING file for the complete license text.
 */
using IEC61850.Server;
using System;
using System.Runtime.InteropServices;

// IEC 61850 API for the libiec61850 .NET wrapper library
namespace IEC61850
{
    // IEC 61850 server API.
    namespace Model
    {
        public enum SMVEvent
        {
            IEC61850_SVCB_EVENT_ENABLE = 1,
            IEC61850_SVCB_EVENT_DISABLE = 0,
        }

        public class SVControlBlock : ModelNode
        {
            private IntPtr self = IntPtr.Zero;
            public IedModel parent { get; }
            internal IntPtr Self { get => self; }

            [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
            static extern IntPtr SVControlBlock_create(string name, IntPtr parent, string svID, string dataSet, UInt32 confRev, uint smpMod,
            UInt16 smpRate, uint optFlds, bool isUnicast);

            [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
            static extern IntPtr SVControlBlock_getName(IntPtr self);

            /// <summary>
            /// create a new Multicast/Unicast Sampled Value (SV) control block (SvCB)
            /// Create a new Sampled Value control block(SvCB) and add it to the given logical node(LN)
            /// </summary>
            /// <param name="name">name of the SvCB relative to the parent LN</param>
            /// <param name="parent">the parent LN</param>
            /// <param name="svID">the application ID of the SvCB</param>
            /// <param name="dataSet">the data set reference to be used by the SVCB</param>
            /// <param name="confRev">the configuration revision</param>
            /// <param name="smpMod">the sampling mode used</param>
            /// <param name="smpRate">the sampling rate used</param>
            /// <param name="optFlds"></param>
            /// <param name="isUnicast">the optional element configuration</param>
            public SVControlBlock(string name, IedModel parent, string svID, string dataSet, UInt32 confRev, uint smpMod,
                UInt16 smpRate, uint optFlds, bool isUnicast)
            {
                self = SVControlBlock_create(name, parent.self, svID, dataSet, confRev, smpMod, smpRate, optFlds, isUnicast);
                this.parent = parent;
            }

            /// <summary>
            /// create a new Multicast/Unicast Sampled Value (SV) control block (SvCB)
            /// Create a new Sampled Value control block(SvCB) and add it to the given logical node(LN)
            /// </summary>
            /// <param name="self">the svcontrol instance</param>
            public SVControlBlock(IntPtr self)
            {
                this.self = self;
            }

            public string Name
            {
                get
                {
                    return Marshal.PtrToStringAnsi(SVControlBlock_getName(self));
                }
            }

        }
    }

}
