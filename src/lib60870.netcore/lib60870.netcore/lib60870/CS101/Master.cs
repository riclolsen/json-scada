/*
 *  Master.cs
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
    /// <summary>
    /// Handler that is called when a new ASDU is received
    /// </summary>
	public delegate bool ASDUReceivedHandler(object parameter,int slaveAddress,ASDU asdu);

    /// <summary>
    /// Common interface for CS104 and CS101 balanced and unbalanced master
    /// </summary>
    public abstract class Master
    {

        protected bool debugOutput;

        public bool DebugOutput
        {
            get
            {
                return this.debugOutput;
            }
            set
            {
                debugOutput = value;
            }
        }

        /// <summary>
        /// Sends the interrogation command.
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qoi">Qualifier of interrogation (20 = station interrogation)</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendInterrogationCommand(CauseOfTransmission cot, int ca, byte qoi);

        /// <summary>
        /// Sends the counter interrogation command (C_CI_NA_1 typeID: 101)
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qcc">Qualifier of counter interrogation command</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendCounterInterrogationCommand(CauseOfTransmission cot, int ca, byte qcc);

        /// <summary>
        /// Sends a read command (C_RD_NA_1 typeID: 102).
        /// </summary>
        /// 
        /// This will send a read command C_RC_NA_1 (102) to the slave/outstation. The COT is always REQUEST (5).
        /// It is used to implement the cyclical polling of data application function.
        /// 
        /// <param name="ca">Common address</param>
        /// <param name="ioa">Information object address</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendReadCommand(int ca, int ioa);

        /// <summary>
        /// Sends a clock synchronization command (C_CS_NA_1 typeID: 103).
        /// </summary>
        /// <param name="ca">Common address</param>
        /// <param name="time">the new time to set</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendClockSyncCommand(int ca, CP56Time2a time);

        /// <summary>
        /// Sends a test command (C_TS_NA_1 typeID: 104).
        /// </summary>
        /// 
        /// Not required and supported by IEC 60870-5-104. 
        /// 
        /// <param name="ca">Common address</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendTestCommand(int ca);

        /// <summary>
        /// Sends a test command with CP56Time2a time (C_TS_TA_1 typeID: 107).
        /// </summary>
        /// <param name="ca">Common address</param>
        /// <param name="tsc">test sequence number</param>
        /// <param name="time">test timestamp</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendTestCommandWithCP56Time2a(int ca, ushort tsc, CP56Time2a time);

        /// <summary>
        /// Sends a reset process command (C_RP_NA_1 typeID: 105).
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qrp">Qualifier of reset process command</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendResetProcessCommand(CauseOfTransmission cot, int ca, byte qrp);

        /// <summary>
        /// Sends a delay acquisition command (C_CD_NA_1 typeID: 106).
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="delay">delay for acquisition</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendDelayAcquisitionCommand(CauseOfTransmission cot, int ca, CP16Time2a delay);

        /// <summary>
        /// Sends the control command.
        /// </summary>
        /// 
        /// The type ID has to match the type of the InformationObject!
        /// 
        /// C_SC_NA_1 -> SingleCommand
        /// C_DC_NA_1 -> DoubleCommand
        /// C_RC_NA_1 -> StepCommand
        /// C_SC_TA_1 -> SingleCommandWithCP56Time2a
        /// C_SE_NA_1 -> SetpointCommandNormalized
        /// C_SE_NB_1 -> SetpointCommandScaled
        /// C_SE_NC_1 -> SetpointCommandShort
        /// C_BO_NA_1 -> Bitstring32Command
        /// 
        /// <param name="cot">Cause of transmission (use ACTIVATION to start a control sequence)</param>
        /// <param name="ca">Common address</param>
        /// <param name="sc">Information object of the command</param>
        /// <exception cref="ConnectionException">description</exception>
        public abstract void SendControlCommand(CauseOfTransmission cot, int ca, InformationObject sc);

        /// <summary>
        /// Sends an arbitrary ASDU to the connected slave
        /// </summary>
        /// <param name="asdu">The ASDU to send</param>
        public abstract void SendASDU(ASDU asdu);


        /// <summary>
        /// Read the file from slave (upload file)
        /// </summary>
        /// <param name="ca">CA</param>
        /// <param name="ioa">IOA</param>
        /// <param name="nof">Name of file (file type)</param>
        /// <param name="receiver">file receiver instance</param>
        public abstract void GetFile(int ca, int ioa, NameOfFile nof, IFileReceiver receiver);

        /// <summary>
        /// Sends the file to slave (download file)
        /// </summary>
        /// <param name="ca">CA</param>
        /// <param name="ioa">IOA</param>
        /// <param name="nof">Name of file (file type)</param>
        /// <param name="fileProvider">File provider instance</param>
        public abstract void SendFile (int ca, int ioa, NameOfFile nof, IFileProvider fileProvider);

        /// <summary>
        /// Get the application layer parameters used by this master instance
        /// </summary>
        /// <returns>used application layer parameters</returns>
        public abstract ApplicationLayerParameters GetApplicationLayerParameters();

        /// <summary>
        /// Sets the raw message handler for received messages
        /// </summary>
        /// <param name="handler">Handler/delegate that will be invoked when a message is received</param>
        /// <param name="parameter">will be passed to the delegate</param>
        public abstract void SetReceivedRawMessageHandler(RawMessageHandler handler, object parameter);

        /// <summary>
        /// Sets the sent message handler for sent messages.
        /// </summary>
        /// <param name="handler">Handler/delegate that will be invoked when a message is sent<</param>
        /// <param name="parameter">will be passed to the delegate</param>
        public abstract void SetSentRawMessageHandler(RawMessageHandler handler, object parameter);

    }
		
}

