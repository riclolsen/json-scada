/*
 *  PrimaryLinkLayer.cs
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
using System.Collections.Generic;

namespace lib60870.linklayer
{
    public class LinkLayerBusyException : lib60870.ConnectionException
    {
        public LinkLayerBusyException(string message)
            : base(message)
        {
        }

        public LinkLayerBusyException(string message, Exception e)
            : base(message, e)
        {
        }
    }

    internal interface IPrimaryLinkLayerCallbacks
    {

        /// <summary>
        /// Indicate an access demand request form the client (ACD bit set in response)
        /// </summary>
        /// <param name="slaveAddress">address of the slave that requested the access demand</param>
        void AccessDemand(int slaveAddress);

        /// <summary>
        /// User data (application layer data) received from a slave
        /// </summary>
        /// <param name="slaveAddress">address of the slave that sent the data</param>
        /// <param name="message">buffer containing the received message</param>
        /// <param name="start">start of user data in the buffer</param>
        /// <param name="length">length of user data in the buffer</param>
        void UserData(int slaveAddress, byte[] message, int start, int length);

        /// <summary>
        /// A former request to the slave (UD Class 1, UD Class 2, confirmed...) resulted in a timeout
        /// Station does not respond indication
        /// </summary>
        /// <param name="slaveAddress">address of the slave that caused the timeout</param>
        void Timeout(int slaveAddress);
    }

    internal abstract class PrimaryLinkLayer
    {
        public abstract void HandleMessage(FunctionCodeSecondary fcs, bool dir, bool dfc, 
                                     int address, byte[] msg, int userDataStart, int userDataLength);

        public abstract void RunStateMachine();

        public abstract void SendLinkLayerTestFunction();
    }

}
