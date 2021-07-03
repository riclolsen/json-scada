/*
 *  LinkLayer.cs
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
using System.IO.Ports;

namespace lib60870.linklayer
{
    /// <summary>
    /// Will be called by the stack when the state of a link layer connection changes
    /// </summary>
    /// <param name="address">Address of the slave (only used for unbalanced master mode)</param>
	public delegate void LinkLayerStateChanged(object parameter,int address,LinkLayerState newState);

    public enum LinkLayerState
    {
        IDLE,
        ERROR,
        BUSY,
        AVAILABLE
    }

    public enum LinkLayerMode
    {
        UNBALANCED,
        BALANCED
    }

    /* Function codes for unbalanced transmission */
    internal enum FunctionCodePrimary
    {
        RESET_REMOTE_LINK = 0,
        /* Reset CU (communication unit) */
        RESET_USER_PROCESS = 1,
        TEST_FUNCTION_FOR_LINK = 2,
        USER_DATA_CONFIRMED = 3,
        USER_DATA_NO_REPLY = 4,
        RESET_FCB = 7,
        /* required/only for CS103 */
        REQUEST_FOR_ACCESS_DEMAND = 8,
        REQUEST_LINK_STATUS = 9,
        REQUEST_USER_DATA_CLASS_1 = 10,
        REQUEST_USER_DATA_CLASS_2 = 11
    }

    /* Function codes for unbalanced transmission */
    internal enum FunctionCodeSecondary
    {
        ACK = 0,
        NACK = 1,
        RESP_USER_DATA = 8,
        RESP_NACK_NO_DATA = 9,
        STATUS_OF_LINK_OR_ACCESS_DEMAND = 11,
        LINK_SERVICE_NOT_FUNCTIONING = 14,
        LINK_SERVICE_NOT_IMPLEMENTED = 15
    }

    /// <summary>
    /// Link layer specific parameters.
    /// </summary>
    public class LinkLayerParameters
    {
        private int addressLength = 1;
        /* 0/1/2 bytes */
        private int timeoutForACK = 1000;
        /* timeout for ACKs in ms */
        private long timeoutRepeat = 1000;
        /* timeout for repeating messages when no ACK received in ms */
        private bool useSingleCharACK = true;
        /* use single char ACK for ACK (FC=0) or RESP_NO_USER_DATA (FC=9) */

        /// <summary>
        /// Gets or sets the length of the link layer address field
        /// </summary>
        /// <para>The value can be either 0, 1, or 2 for balanced mode or 0, or 1 for unbalanced mode</para>
        /// <value>The length of the address in byte</value>
        public int AddressLength
        {
            get
            {
                return this.addressLength;
            }
            set
            {
                addressLength = value;
            }
        }

        /// <summary>
        /// Gets or sets the timeout for message ACK
        /// </summary>
        /// <value>The timeout to wait for message ACK in ms</value>
        public int TimeoutForACK
        {
            get
            {
                return this.timeoutForACK;
            }
            set
            {
                timeoutForACK = value;
            }
        }

        /// <summary>
        /// Gets or sets the timeout for message repetition in case of missing ACK messages
        /// </summary>
        /// <value>The timeout for message repetition in ms</value>
        public long TimeoutRepeat
        {
            get
            {
                return this.timeoutRepeat;
            }
            set
            {
                timeoutRepeat = value;
            }
        }

        /// <summary>
        /// Gets or sets a value indicating whether the secondary link layer uses single character ACK instead of FC 0 or FC 9
        /// </summary>
        /// <value><c>true</c> if use single char ACK; otherwise, <c>false</c>.</value>
        public bool UseSingleCharACK
        {
            get
            {
                return this.useSingleCharACK;
            }
            set
            {
                this.useSingleCharACK = value;
            }
        }
    }


    internal enum PrimaryLinkLayerState
    {
        IDLE,
        EXECUTE_REQUEST_STATUS_OF_LINK,
        EXECUTE_RESET_REMOTE_LINK,
        LINK_LAYERS_AVAILABLE,
        EXECUTE_SERVICE_SEND_CONFIRM,
        EXECUTE_SERVICE_REQUEST_RESPOND,
        SECONDARY_LINK_LAYER_BUSY
        /* Only required in balanced link layer */
    }


    internal class LinkLayer
    {
        protected Action<string> DebugLog;
	
        protected byte[] buffer;
        /* byte buffer to receice and send frames */

        public LinkLayerParameters linkLayerParameters;
        protected SerialTransceiverFT12 transceiver;
        private LinkLayerMode linkLayerMode = LinkLayerMode.BALANCED;

        private PrimaryLinkLayer primaryLinkLayer = null;
        private SecondaryLinkLayer secondaryLinkLayer = null;

        private byte[] SINGLE_CHAR_ACK = new byte[] { 0xe5 };

        private bool dir;
        /* ONLY for balanced link layer */

        private RawMessageHandler receivedRawMessageHandler = null;
        private object receivedRawMessageHandlerParameter = null;

        private RawMessageHandler sentRawMessageHandler = null;
        private object sentRawMessageHandlerParameter = null;

        public LinkLayer(byte[] buffer, LinkLayerParameters parameters, SerialTransceiverFT12 transceiver, Action<string> debugLog)
        {
            this.buffer = buffer;
            this.linkLayerParameters = parameters;
            this.transceiver = transceiver;
            this.DebugLog = debugLog;
        }

        public void SetReceivedRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            receivedRawMessageHandler = handler;
            receivedRawMessageHandlerParameter = parameter;
        }

        public void SetSentRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            sentRawMessageHandler = handler;
            sentRawMessageHandlerParameter = parameter;
        }

        internal int GetBroadcastAddress()
        {
            if (linkLayerParameters.AddressLength == 1)
            {
                return 255;
            }
            else if (linkLayerParameters.AddressLength == 2)
            {
                return 65535;
            }

            return 0;
        }


        public int OwnAddress
        {
            get
            {
                return secondaryLinkLayer.Address;
            }
            set
            {
                secondaryLinkLayer.Address = value;
            }
        }


        /// <summary>
        /// Gets or sets a value indicating whether this balanced <see cref="lib60870.CS103.LinkLayer"/> has DIR bit set
        /// </summary>
        /// <value><c>true</c> if DI; otherwise, <c>false</c>.</value>
        public bool DIR
        {
            get
            {
                return this.dir;
            }
            set
            {
                dir = value;
            }
        }

        public long TimeoutForACK
        {
            get { return linkLayerParameters.TimeoutForACK; }
        }

        public long TimeoutRepeat
        {
            get { return linkLayerParameters.TimeoutRepeat; }
        }

        public void SetPrimaryLinkLayer(PrimaryLinkLayer primaryLinkLayer)
        {
            this.primaryLinkLayer = primaryLinkLayer;
        }

        public void SetSecondaryLinkLayer(SecondaryLinkLayer secondaryLinkLayer)
        {
            this.secondaryLinkLayer = secondaryLinkLayer;
        }

        public LinkLayerMode LinkLayerMode
        {
            get
            {
                return this.linkLayerMode;
            }
            set
            {
                linkLayerMode = value;
            }
        }

        public void SendTestFunction()
        {
            if (primaryLinkLayer != null)
                primaryLinkLayer.SendLinkLayerTestFunction();
        }

        public void SendSingleCharACK()
        {
            if (sentRawMessageHandler != null)
                sentRawMessageHandler(sentRawMessageHandlerParameter, SINGLE_CHAR_ACK, 1);

            transceiver.SendMessage(SINGLE_CHAR_ACK, 1);
        }


        public void SendFixedFramePrimary(FunctionCodePrimary fc, int address, bool fcb, bool fcv)
        {
            SendFixedFrame((byte)fc, address, true, dir, fcb, fcv);
        }

        public void SendFixedFrameSecondary(FunctionCodeSecondary fc, int address, bool acd, bool dfc)
        {
            SendFixedFrame((byte)fc, address, false, dir, acd, dfc);
        }

        public void SendFixedFrame(byte fc, int address, bool prm, bool dir, bool acd, bool dfc)
        {
            int bufPos = 0;

            buffer[bufPos++] = 0x10; /* START */

            byte c = fc;

            if (prm)
                c += 0x40;

            if (dir)
                c += 0x80;

            if (acd)
                c += 0x20;

            if (dfc)
                c += 0x10;

            buffer[bufPos++] = (byte)c;

            if (linkLayerParameters.AddressLength > 0)
            {
                buffer[bufPos++] = (byte)(address % 0x100);

                if (linkLayerParameters.AddressLength > 1)
                    buffer[bufPos++] = (byte)((address / 0x100) % 0x100);			
            }

            byte checksum = 0;

            for (int i = 1; i < bufPos; i++)
                checksum += buffer[i];

            buffer[bufPos++] = checksum;

            buffer[bufPos++] = 0x16; /* END */

            if (sentRawMessageHandler != null)
                sentRawMessageHandler(sentRawMessageHandlerParameter, buffer, bufPos);

            transceiver.SendMessage(buffer, bufPos);
        }


        public void SendVariableLengthFramePrimary(FunctionCodePrimary fc, int address, bool fcb, bool fcv, BufferFrame frame)
        {
            buffer[0] = 0x68; /* START */
            buffer[3] = 0x68; /* START */

            byte c = (byte)fc;

            if (dir)
                c += 0x80;

            c += 0x40; // PRM = 1;

            if (fcv)
                c += 0x10;

            if (fcb)
                c += 0x20;

            buffer[4] = c;

            int bufPos = 5;

            if (linkLayerParameters.AddressLength > 0)
            {
                buffer[bufPos++] = (byte)(address % 0x100);

                if (linkLayerParameters.AddressLength > 1)
                    buffer[bufPos++] = (byte)((address / 0x100) % 0x100);			
            }

            byte[] userData = frame.GetBuffer();
            int userDataLength = frame.GetMsgSize();

            for (int i = 0; i < userDataLength; i++)
                buffer[bufPos++] = userData[i];

            int l = 1 + linkLayerParameters.AddressLength + frame.GetMsgSize();

            if (l > 255)
                return;

            buffer[1] = (byte)l;
            buffer[2] = (byte)l;

            byte checksum = 0;

            for (int i = 4; i < bufPos; i++)
                checksum += buffer[i];

            buffer[bufPos++] = checksum;

            buffer[bufPos++] = 0x16; /* END */

            if (sentRawMessageHandler != null)
                sentRawMessageHandler(sentRawMessageHandlerParameter, buffer, bufPos);

            transceiver.SendMessage(buffer, bufPos);
        }

        internal void SendVariableLengthFrameSecondary(FunctionCodeSecondary fc, int address, bool acd, bool dfc, BufferFrame frame)
        {
            buffer[0] = 0x68; /* START */
            buffer[3] = 0x68; /* START */

            byte c = (byte)((int)fc & 0x1f);

            if (linkLayerMode == LinkLayerMode.BALANCED)
            {
                if (dir)
                    c += 0x80;
            }

            if (acd)
                c += 0x20;

            if (dfc)
                c += 0x10;

            buffer[4] = c;

            int bufPos = 5;

            if (linkLayerParameters.AddressLength > 0)
            {
                buffer[bufPos++] = (byte)(address % 0x100);

                if (linkLayerParameters.AddressLength > 1)
                    buffer[bufPos++] = (byte)((address / 0x100) % 0x100);			
            }

            byte[] userData = frame.GetBuffer();
            int userDataLength = frame.GetMsgSize();

            int l = 1 + linkLayerParameters.AddressLength + userDataLength;

            if (l > 255)
                return;

            buffer[1] = (byte)l;
            buffer[2] = (byte)l;

            for (int i = 0; i < userDataLength; i++)
                buffer[bufPos++] = userData[i];

            byte checksum = 0;

            for (int i = 4; i < bufPos; i++)
                checksum += buffer[i];

            buffer[bufPos++] = checksum;

            buffer[bufPos++] = 0x16; /* END */

            if (sentRawMessageHandler != null)
                sentRawMessageHandler(sentRawMessageHandlerParameter, buffer, bufPos);

            transceiver.SendMessage(buffer, bufPos);
        }


        private void ParseHeaderSecondaryUnbalanced(byte[] msg, int msgSize)
        {			
            int userDataLength = 0;
            int userDataStart = 0;
            byte c;
            int csStart;
            int csIndex;
            int address = 0;

            if (msg[0] == 0x68)
            {

                if (msg[1] != msg[2])
                {
                    DebugLog("ERROR: L fields differ!");
                    return;
                }

                userDataLength = (int)msg[1] - linkLayerParameters.AddressLength - 1;
                userDataStart = 5 + linkLayerParameters.AddressLength;

                csStart = 4;
                csIndex = userDataStart + userDataLength;

                // check if message size is reasonable
                if (msgSize != (userDataStart + userDataLength + 2 /* CS + END */))
                {
                    DebugLog("ERROR: Invalid message length");
                    return;
                }

                c = msg[4];
            }
            else if (msg[0] == 0x10)
            {
                c = msg[1];
                csStart = 1;
                csIndex = 2 + linkLayerParameters.AddressLength;

            }
            else if (msg[0] == 0xE5)
            {
                /* Confirmation message from other slave --> ignore */
                return;
            }
            else
            {
                DebugLog("ERROR: Received unexpected message type in unbalanced slave mode!");
                return;
            }

            bool isBroadcast = false;

            //check address
            if (linkLayerParameters.AddressLength > 0)
            {
                address = msg[csStart + 1];

                if (linkLayerParameters.AddressLength > 1)
                {
                    address += (msg[csStart + 2] * 0x100);

                    if (address == 65535)
                        isBroadcast = true;
                }
                else
                {
                    if (address == 255)
                        isBroadcast = true;
                }
            }

            int fc = c & 0x0f;
            FunctionCodePrimary fcp = (FunctionCodePrimary)fc;

            if (isBroadcast)
            {
                if (fcp != FunctionCodePrimary.USER_DATA_NO_REPLY)
                {
                    DebugLog("ERROR: Invalid function code for broadcast message!");
                    return;
                }

            }
            else
            {
                if (address != secondaryLinkLayer.Address)
                {
                    DebugLog("INFO: unknown link layer address -> ignore message");
                    return;
                }
            }

            //check checksum
            byte checksum = 0;

            for (int i = csStart; i < csIndex; i++)
                checksum += msg[i];

            if (checksum != msg[csIndex])
            {
                DebugLog("ERROR: checksum invalid!");
                return;
            }


            // parse C field bits
            bool prm = ((c & 0x40) == 0x40);

            if (prm == false)
            {
                DebugLog("ERROR: Received secondary message in unbalanced slave mode!");
                return;
            }
				
            bool fcb = ((c & 0x20) == 0x20);
            bool fcv = ((c & 0x10) == 0x10);

            DebugLog("PRM=" + (prm == true ? "1" : "0") + " FCB=" + (fcb == true ? "1" : "0") + " FCV=" + (fcv == true ? "1" : "0")
                + " FC=" + fc + "(" + fcp.ToString() + ")");

            if (secondaryLinkLayer != null)
                secondaryLinkLayer.HandleMessage(fcp, isBroadcast, address, fcb, fcv, msg, userDataStart, userDataLength);
            else
                DebugLog("No secondary link layer available!");
        }


        public void HandleMessageBalancedAndPrimaryUnbalanced(byte[] msg, int msgSize)
        {
            int userDataLength = 0;
            int userDataStart = 0;
            byte c = 0;
            int csStart = 0;
            int csIndex = 0;
            int address = 0; /* address can be ignored in balanced mode? */
            bool prm = true;
            int fc = 0;

            bool isAck = false;

            if (msg[0] == 0x68)
            {

                if (msg[1] != msg[2])
                {
                    DebugLog("ERROR: L fields differ!");
                    return;
                }

                userDataLength = (int)msg[1] - linkLayerParameters.AddressLength - 1;
                userDataStart = 5 + linkLayerParameters.AddressLength;

                csStart = 4;
                csIndex = userDataStart + userDataLength;

                // check if message size is reasonable
                if (msgSize != (userDataStart + userDataLength + 2 /* CS + END */))
                {
                    DebugLog("ERROR: Invalid message length");
                    return;
                }

                c = msg[4];

                if (linkLayerParameters.AddressLength > 0)
                    address += msg[5];

                if (linkLayerParameters.AddressLength > 1)
                    address += msg[6] * 0x100;
            }
            else if (msg[0] == 0x10)
            {
                c = msg[1];
                csStart = 1;
                csIndex = 2 + linkLayerParameters.AddressLength;

                if (linkLayerParameters.AddressLength > 0)
                    address += msg[2];

                if (linkLayerParameters.AddressLength > 1)
                    address += msg[3] * 0x100;

            }
            else if (msg[0] == 0xe5)
            {
                isAck = true;
                fc = (int)FunctionCodeSecondary.ACK;
                prm = false; /* single char ACK is only sent by secondary station */
                DebugLog("Received single char ACK");
            }
            else
            {
                DebugLog("ERROR: Received unexpected message type!");
                return;
            }

            if (isAck == false)
            {

                //check checksum
                byte checksum = 0;

                for (int i = csStart; i < csIndex; i++)
                    checksum += msg[i];

                if (checksum != msg[csIndex])
                {
                    DebugLog("ERROR: checksum invalid!");
                    return;
                }

                // parse C field bits
                fc = c & 0x0f;
                prm = ((c & 0x40) == 0x40);

                if (prm)
                { /* we are secondary link layer */
                    bool fcb = ((c & 0x20) == 0x20);
                    bool fcv = ((c & 0x10) == 0x10);

                    DebugLog("PRM=" + (prm == true ? "1" : "0") + " FCB=" + (fcb == true ? "1" : "0") + " FCV=" + (fcv == true ? "1" : "0")
                        + " FC=" + fc + "(" + ((FunctionCodePrimary)c).ToString() + ")");

                    FunctionCodePrimary fcp = (FunctionCodePrimary)fc;

                    if (secondaryLinkLayer != null)
                        secondaryLinkLayer.HandleMessage(fcp, false, address, fcb, fcv, msg, userDataStart, userDataLength);
                    else
                        DebugLog("No secondary link layer available!");

                }
                else
                { /* we are primary link layer */ 
                    bool dir = ((c & 0x80) == 0x80); /* DIR - direction for balanced transmission */
                    bool dfc = ((c & 0x10) == 0x10); /* DFC - Data flow control */
                    bool acd = ((c & 0x20) == 0x20); /* ACD - access demand for class 1 data - for unbalanced transmission */

                    DebugLog("PRM=" + (prm == true ? "1" : "0") + " DIR=" + (dir == true ? "1" : "0") + " DFC=" + (dfc == true ? "1" : "0")
                        + " FC=" + fc + "(" + ((FunctionCodeSecondary)c).ToString() + ")");

                    FunctionCodeSecondary fcs = (FunctionCodeSecondary)fc;

                    if (primaryLinkLayer != null)
                    {

                        if (linkLayerMode == LinkLayerMode.BALANCED)
                            primaryLinkLayer.HandleMessage(fcs, dir, dfc, address, msg, userDataStart, userDataLength);
                        else
                            primaryLinkLayer.HandleMessage(fcs, acd, dfc, address, msg, userDataStart, userDataLength);
                    }
                    else
                        DebugLog("No primary link layer available!");

                }
			
            }
            else
            { /* Single byte ACK */
                if (primaryLinkLayer != null)
                    primaryLinkLayer.HandleMessage(FunctionCodeSecondary.ACK, false, false, -1, null, 0, 0);
            }

        }

        void HandleMessageAction(byte[] msg, int msgSize)
        {
            DebugLog("RECV " + BitConverter.ToString(msg, 0, msgSize));

            bool handleMessage = true;

            if (receivedRawMessageHandler != null)
                handleMessage = receivedRawMessageHandler(receivedRawMessageHandlerParameter, msg, msgSize);

            if (handleMessage)
            {

                if (linkLayerMode == LinkLayerMode.BALANCED)
                    HandleMessageBalancedAndPrimaryUnbalanced(buffer, msgSize);
                else
                {
                    if (secondaryLinkLayer != null)
                        ParseHeaderSecondaryUnbalanced(buffer, msgSize);
                    else if (primaryLinkLayer != null)
                        HandleMessageBalancedAndPrimaryUnbalanced(buffer, msgSize);
                    else
                        DebugLog("ERROR: Neither primary nor secondary link layer available!");
                }
            }
            else
                DebugLog("Message ignored because of raw message handler");
        }

        public void Run()
        {
            try
            {
                transceiver.ReadNextMessage(buffer, HandleMessageAction);
            }
            catch (InvalidOperationException)
            {
                //TODO exception handling code       
            }

            if (linkLayerMode == LinkLayerMode.BALANCED)
            {
                primaryLinkLayer.RunStateMachine();
                secondaryLinkLayer.RunStateMachine();
            }
            else
            {
                if (primaryLinkLayer != null)
                    primaryLinkLayer.RunStateMachine();
                else if (secondaryLinkLayer != null)
                    secondaryLinkLayer.RunStateMachine();				
            }

        }

       public void AddPortDeniedHandler (EventHandler eventHandler)
       {
            transceiver.AddPortDeniedHandler(eventHandler);
       }
    }
}

