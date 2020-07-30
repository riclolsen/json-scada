/*
 *  SecondaryLinkLayer.cs
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

namespace lib60870.linklayer
{

    internal interface ISecondaryApplicationLayer
    {
        bool IsClass1DataAvailable();

        BufferFrame GetClass1Data();

        BufferFrame GetCLass2Data();

        bool HandleReceivedData(byte[] msg, bool isBroadcast, int userDataStart, int userDataLength);

        void ResetCUReceived(bool onlyFCB);
    }

    internal abstract class SecondaryLinkLayer
    {
        public abstract int Address
        {
            get;
            set;
        }

        public abstract void HandleMessage(FunctionCodePrimary fcp, bool isBroadcast, int address, bool fcb, bool fcv, byte[] msg, int userDataStart, int userDataLength);

        public abstract void RunStateMachine();
    }

}

