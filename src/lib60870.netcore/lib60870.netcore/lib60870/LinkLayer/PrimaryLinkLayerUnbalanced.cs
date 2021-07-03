/*
  *  Copyright 2016, 2017 MZ Automation GmbH
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
    internal interface IPrimaryLinkLayerUnbalanced
    {
        void ResetCU(int slaveAddress);

        /// <summary>
        /// Determines whether this channel (slave connecrtion) is ready to transmit a new application layer message
        /// </summary>
        /// <returns><c>true</c> if this instance is channel available; otherwise, <c>false</c>.</returns>
        /// <param name="slaveAddress">link layer address of the slave</param>
        bool IsChannelAvailable(int slaveAddress);


        void RequestClass1Data(int slaveAddress);



        void RequestClass2Data(int slaveAddress);


        void SendConfirmed(int slaveAddress, BufferFrame message);

        void SendNoReply(int slaveAddress, BufferFrame message);
    }


    internal class PrimaryLinkLayerUnbalanced : PrimaryLinkLayer, IPrimaryLinkLayerUnbalanced
    {
        private LinkLayer linkLayer;
        private Action<string> DebugLog;

        //	private bool waitingForResponse = false;

        private List<SlaveConnection> slaveConnections;

        /// <summary>
        /// The current active slave connection.
        /// </summary>
        private SlaveConnection currentSlave = null;

        private BufferFrame nextBroadcastMessage = null;

        private IPrimaryLinkLayerCallbacks callbacks = null;

        private LinkLayerStateChanged stateChanged = null;
        private object stateChangedParameter = null;

        // can this class implement Master interface?
        private class SlaveConnection
        {

            private Action<string> DebugLog = null;

            public int address;
            public PrimaryLinkLayerState primaryState = PrimaryLinkLayerState.IDLE;
            public long lastSendTime = 0;
            public long originalSendTime = 0;
            public bool nextFcb = true;
            public bool waitingForResponse = false;
            public LinkLayerState linkLayerState = LinkLayerState.IDLE;

            PrimaryLinkLayerUnbalanced linkLayerUnbalanced;

            private bool sendLinkLayerTestFunction = false;

            // don't send new application layer messages to avoid data flow congestion
            private bool dontSendMessages = false;

            public BufferFrame nextMessage = null;
            private BufferFrame lastSentASDU = null;

            public bool requireConfirmation = false;

            public bool resetCu = false;
            public bool requestClass2Data = false;
            public bool requestClass1Data = false;

            private LinkLayer linkLayer;

            private void SetState(LinkLayerState newState)
            {
                if (linkLayerState != newState)
                {

                    linkLayerState = newState;

                    if (linkLayerUnbalanced.stateChanged != null)
                        linkLayerUnbalanced.stateChanged(linkLayerUnbalanced.stateChangedParameter,
                            address, newState);
                }
            }

            public SlaveConnection(int address, LinkLayer linkLayer, Action<string> debugLog, PrimaryLinkLayerUnbalanced linkLayerUnbalanced)
            {
                this.address = address;
                this.linkLayer = linkLayer;
                this.DebugLog = debugLog;
                this.linkLayerUnbalanced = linkLayerUnbalanced;
            }

            public bool IsMessageWaitingToSend()
            {
                if (requestClass1Data || requestClass2Data || (nextMessage != null))
                    return true;
                else
                    return false;
            }

            internal void HandleMessage(FunctionCodeSecondary fcs, bool acd, bool dfc, 
                               int addr, byte[] msg, int userDataStart, int userDataLength)
            {
                PrimaryLinkLayerState newState = primaryState;

                if (dfc)
                {

                    //stop sending ASDUs; only send Status of link requests
                    dontSendMessages = true;

                    switch (primaryState)
                    {
                        case PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK:
                        case PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK:
                            newState = PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK;
                            break;
                        case PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM:
						//TODO message must be handled and switched to BUSY state later!
                        case PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY:
                            newState = PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY;
                            break;
                    }

                    SetState(LinkLayerState.BUSY);

                    primaryState = newState;
                    return;

                }
                else
                {
                    // unblock transmission of application layer messages
                    dontSendMessages = false;
                }

                switch (fcs)
                {

                    case FunctionCodeSecondary.ACK:

                        DebugLog("[SLAVE " + address + "] PLL - received ACK");

                        if (primaryState == PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK)
                        {
                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
					
                            SetState(LinkLayerState.AVAILABLE);
                        }
                        else if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                        {

                            if (sendLinkLayerTestFunction)
                                sendLinkLayerTestFunction = false;

                            SetState(LinkLayerState.AVAILABLE);	  

                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        }
                        else if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_REQUEST_RESPOND)
                        {

                            /* single char ACK is interpreted as RESP NO DATA */
                            requestClass1Data = false;
                            requestClass2Data = false;

                            SetState(LinkLayerState.AVAILABLE);

                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        }

                        waitingForResponse = false;
                        break;

                    case FunctionCodeSecondary.NACK:
					
                        DebugLog("[SLAVE " + address + "] PLL - received NACK");
				
                        if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                        {
					
                            SetState(LinkLayerState.BUSY);
					
                            newState = PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY;
                        }

                        waitingForResponse = false;
                        break;

                    case FunctionCodeSecondary.STATUS_OF_LINK_OR_ACCESS_DEMAND:
					
                        DebugLog("[SLAVE " + address + "] PLL - received STATUS OF LINK");

                        if (primaryState == PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK)
                        {
						
                            DebugLog("[SLAVE " + address + "] PLL - SEND RESET REMOTE LINK");

                            linkLayer.SendFixedFramePrimary(FunctionCodePrimary.RESET_REMOTE_LINK, address, false, false);

                            nextFcb = true;
                            lastSendTime = SystemUtils.currentTimeMillis();
                            waitingForResponse = true;
                            newState = PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK;

                            SetState(LinkLayerState.BUSY);
                        }
                        else
                        { /* illegal message */
                            newState = PrimaryLinkLayerState.IDLE;
					
                            SetState(LinkLayerState.ERROR);

                            waitingForResponse = false;
                        }

                        break;

                    case FunctionCodeSecondary.RESP_USER_DATA:
					
                        DebugLog("[SLAVE " + address + "] PLL - received USER DATA");

                        if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_REQUEST_RESPOND)
                        {
                            linkLayerUnbalanced.callbacks.UserData(address, msg, userDataStart, userDataLength);

                            requestClass1Data = false;
                            requestClass2Data = false;

                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;

                            SetState(LinkLayerState.AVAILABLE);
                        }
                        else
                        { /* illegal message */
                            newState = PrimaryLinkLayerState.IDLE;

                            SetState(LinkLayerState.ERROR);
                        }

                        waitingForResponse = false;

                        break;

                    case FunctionCodeSecondary.RESP_NACK_NO_DATA:
					
                        DebugLog("[SLAVE " + address + "] PLL - received RESP NO DATA");

                        if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_REQUEST_RESPOND)
                        {
                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;

                            requestClass1Data = false;
                            requestClass2Data = false;

                            SetState(LinkLayerState.AVAILABLE);
                        }
                        else
                        { /* illegal message */
                            newState = PrimaryLinkLayerState.IDLE;
					
                            SetState(LinkLayerState.ERROR);
                        }

                        waitingForResponse = false;

                        break;

                    case FunctionCodeSecondary.LINK_SERVICE_NOT_FUNCTIONING:
                    case FunctionCodeSecondary.LINK_SERVICE_NOT_IMPLEMENTED:
					
                        DebugLog("[SLAVE " + address + "] PLL - link layer service not functioning/not implemented in secondary station ");

                        if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                        {
                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;

                            SetState(LinkLayerState.AVAILABLE);
                        }

                        waitingForResponse = false;

                        break;

                    default:
                        DebugLog("[SLAVE " + address + "] UNEXPECTED SECONDARY LINK LAYER MESSAGE");
                        break;
                }

                if (acd)
                {
                    if (linkLayerUnbalanced.callbacks != null)
                        linkLayerUnbalanced.callbacks.AccessDemand(address);
                }

                DebugLog("[SLAVE " + address + "] PLL RECV - old state: " + primaryState.ToString() + " new state: " + newState.ToString());

                primaryState = newState;
            }

            public void RunStateMachine()
            {
                PrimaryLinkLayerState newState = primaryState;

                long currentTime = SystemUtils.currentTimeMillis();

                switch (primaryState)
                {

                    case PrimaryLinkLayerState.IDLE:

                        waitingForResponse = false;
                        originalSendTime = 0;
                        lastSendTime = 0;
                        sendLinkLayerTestFunction = false;
                        newState = PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK;

                        break;

                    case PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK:

                        if (waitingForResponse)
                        {
						
                            if (currentTime > (lastSendTime + linkLayer.TimeoutForACK))
                            {
                                linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_LINK_STATUS, address, false, false);

                                lastSendTime = SystemUtils.currentTimeMillis();
                            }

                        }
                        else
                        {
						
                            DebugLog("[SLAVE " + address + "] PLL - SEND RESET REMOTE LINK");

                            linkLayer.SendFixedFramePrimary(FunctionCodePrimary.RESET_REMOTE_LINK, address, false, false);

                            lastSendTime = currentTime;
                            originalSendTime = lastSendTime;
                            waitingForResponse = true;

                            nextFcb = true;

                            newState = PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK; 
                        }

                        break;

                    case PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK:

                        if (waitingForResponse)
                        {
                            if (currentTime > (lastSendTime + linkLayer.TimeoutForACK))
                            {
                                waitingForResponse = false;
                                newState = PrimaryLinkLayerState.IDLE;

                                SetState(LinkLayerState.ERROR);
                            }
                        }
                        else
                        {
                            newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;

                            SetState(LinkLayerState.AVAILABLE);
                        }

                        break;

                    case PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE:

                        if (sendLinkLayerTestFunction)
                        {
                            DebugLog("[SLAVE " + address + "] PLL - SEND TEST LINK");

                            linkLayer.SendFixedFramePrimary(FunctionCodePrimary.TEST_FUNCTION_FOR_LINK, address, nextFcb, true);

                            nextFcb = !nextFcb;
                            lastSendTime = currentTime;
                            originalSendTime = lastSendTime;
                            waitingForResponse = true;

                            newState = PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM;
                        }
                        else if (requestClass1Data || requestClass2Data)
                        {

                            if (requestClass1Data)
                            {
                                DebugLog("[SLAVE " + address + "] PLL - SEND FC 10 - REQ UD 1");

                                linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_USER_DATA_CLASS_1, address, nextFcb, true);
                            }
                            else
                            {
                                DebugLog("[SLAVE " + address + "] PLL - SEND FC 11 - REQ UD 2");

                                linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_USER_DATA_CLASS_2, address, nextFcb, true);
                            }

                            nextFcb = !nextFcb;

                            lastSendTime = currentTime;
                            originalSendTime = lastSendTime;
                            waitingForResponse = true;
                            newState = PrimaryLinkLayerState.EXECUTE_SERVICE_REQUEST_RESPOND;
                        }
                        else
                        {

                            if (dontSendMessages == false)
                            {

                                BufferFrame asdu = nextMessage;

                                if (asdu != null)
                                {

                                    DebugLog("[SLAVE " + address + "] PLL - SEND FC 03 - USER DATA CONFIRMED");

                                    linkLayer.SendVariableLengthFramePrimary(FunctionCodePrimary.USER_DATA_CONFIRMED, address, nextFcb, true, asdu);

                                    lastSentASDU = nextMessage;
                                    nextMessage = null;


                                    nextFcb = !nextFcb;

                                    lastSendTime = currentTime;
                                    originalSendTime = lastSendTime;
                                    waitingForResponse = true;

                                    newState = PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM;
                                }
                            }
                        }

                        break;

                    case PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM:

                        if (currentTime > (lastSendTime + linkLayer.TimeoutForACK))
                        {

                            if (currentTime > (originalSendTime + linkLayer.TimeoutRepeat))
                            {
                                DebugLog("[SLAVE " + address + "] TIMEOUT SC: ASDU not confirmed after repeated transmission");
                                newState = PrimaryLinkLayerState.IDLE;

                                SetState(LinkLayerState.ERROR);
                            }
                            else
                            {
                                DebugLog("[SLAVE " + address + "] TIMEOUT SC: 1 ASDU not confirmed");

                                if (sendLinkLayerTestFunction)
                                {

                                    DebugLog("[SLAVE " + address + "] PLL - SEND FC 02 - RESET REMOTE LINK [REPEAT]");

                                    linkLayer.SendFixedFramePrimary(FunctionCodePrimary.TEST_FUNCTION_FOR_LINK, address, !nextFcb, true);
							
                                }
                                else
                                {

                                    DebugLog("[SLAVE " + address + "] PLL - SEND FC 03 - USER DATA CONFIRMED [REPEAT]");

                                    linkLayer.SendVariableLengthFramePrimary(FunctionCodePrimary.USER_DATA_CONFIRMED, address, !nextFcb, true, lastSentASDU);
							
                                }

                                lastSendTime = currentTime;
                            }
                        }

                        break;

                    case PrimaryLinkLayerState.EXECUTE_SERVICE_REQUEST_RESPOND:

                        if (currentTime > (lastSendTime + linkLayer.TimeoutForACK))
                        {

                            if (currentTime > (originalSendTime + linkLayer.TimeoutRepeat))
                            {
                                DebugLog("[SLAVE " + address + "] TIMEOUT: ASDU not confirmed after repeated transmission");
                                newState = PrimaryLinkLayerState.IDLE;
                                requestClass1Data = false;
                                requestClass2Data = false;

                                SetState(LinkLayerState.ERROR);
                            }
                            else
                            {
                                DebugLog("[SLAVE " + address + "] TIMEOUT: ASDU not confirmed");

                                if (requestClass1Data)
                                {
                                    DebugLog("[SLAVE " + address + "] PLL - SEND FC 10 - REQ UD 1 [REPEAT]");

                                    linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_USER_DATA_CLASS_1, address, !nextFcb, true);
                                }
                                else if (requestClass2Data)
                                {

                                    DebugLog("[SLAVE " + address + "] PLL - SEND FC 11 - REQ UD 2 [REPEAT]");

                                    linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_USER_DATA_CLASS_2, address, !nextFcb, true);
                                }
								
                                lastSendTime = currentTime;
                            }
                        }

                        break;

                    case PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY:
					//TODO - reject new requests from application layer?
                        break;

                }

                if (primaryState != newState)
                    DebugLog("[SLAVE " + address + "] PLL - old state: " + primaryState.ToString() + " new state: " + newState.ToString());

                primaryState = newState;

            }
        }

        /********************************
		 * IPrimaryLinkLayerUnbalanced
		 ********************************/


        public void ResetCU(int slaveAddress)
        {				
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave != null)
                slave.resetCu = true;
        }

        public bool IsChannelAvailable(int slaveAddress)
        {
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave != null)
            {
                if (slave.IsMessageWaitingToSend() == false)
                    return true;
            }

            return false;
        }

        public void RequestClass1Data(int slaveAddress)
        {
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave != null)
            {
                slave.requestClass1Data = true;
            }
        }

        public void RequestClass2Data(int slaveAddress)
        {
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave != null)
            {
                if (slave.IsMessageWaitingToSend())
                    throw new LinkLayerBusyException("Message pending");
                else
                    slave.requestClass2Data = true;
            }
        }

        public void SendConfirmed(int slaveAddress, BufferFrame message)
        {
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave != null)
            {
                if (slave.nextMessage != null)
                    throw new LinkLayerBusyException("Message pending");
                else
                {
                    slave.nextMessage = message.Clone();
                    slave.requireConfirmation = true;
                }
            }
        }

        public void SendNoReply(int slaveAddress, BufferFrame message)
        {
            if (slaveAddress == linkLayer.GetBroadcastAddress())
            {
                if (nextBroadcastMessage != null)
                    throw new LinkLayerBusyException("Broadcast message pending");
                else
                    nextBroadcastMessage = message;
            }
            else
            {
                SlaveConnection slave = GetSlaveConnection(slaveAddress);

                if (slave != null)
                {
                    if (slave.IsMessageWaitingToSend())
                        throw new LinkLayerBusyException("Message pending");
                    else
                    {
                        slave.nextMessage = message;
                        slave.requireConfirmation = false;
                    }
                }
            }
        }

        /********************************
         * END IPrimaryLinkLayerUnbalanced
         ********************************/

        public PrimaryLinkLayerUnbalanced(LinkLayer linkLayer, IPrimaryLinkLayerCallbacks callbacks, Action<string> debugLog)
        {
            this.linkLayer = linkLayer;
            this.callbacks = callbacks;
            this.DebugLog = debugLog;
            this.slaveConnections = new List<SlaveConnection>();
        }

        private SlaveConnection GetSlaveConnection(int slaveAddres)
        {
            foreach (SlaveConnection connection in slaveConnections)
            {
                if (connection.address == slaveAddres)
                    return connection;
            }

            return null;
        }

        public void AddSlaveConnection(int slaveAddress)
        {
            SlaveConnection slave = GetSlaveConnection(slaveAddress);

            if (slave == null)
                slaveConnections.Add(new SlaveConnection(slaveAddress, linkLayer, DebugLog, this));
        }

        public LinkLayerState GetStateOfSlave(int slaveAddress)
        {	
            SlaveConnection connection = GetSlaveConnection(slaveAddress);

            if (connection != null)
                return connection.linkLayerState;
            else
                throw new ArgumentException("No slave with this address found");
        }

        public override void HandleMessage(FunctionCodeSecondary fcs, bool acd, bool dfc, 
                                     int address, byte[] msg, int userDataStart, int userDataLength)
        {
            SlaveConnection slave = null;

            if (address == -1)
                slave = currentSlave;
            else
                slave = GetSlaveConnection(address);

            if (slave != null)
            {

                slave.HandleMessage(fcs, acd, dfc, address, msg, userDataStart, userDataLength);

            }
            else
            {
                DebugLog("PLL RECV - response from unknown slave " + address + " !");
            }
        }

        private int currentSlaveIndex = 0;

        public override void RunStateMachine()
        {
            // run all the link layer state machines for the registered slaves

            if (slaveConnections.Count > 0)
            {

                if (currentSlave == null)
                {

                    /* schedule next slave connection */
                    currentSlave = slaveConnections[currentSlaveIndex];
                    currentSlaveIndex = (currentSlaveIndex + 1) % slaveConnections.Count;

                }

                currentSlave.RunStateMachine();

                if (currentSlave.waitingForResponse == false)
                    currentSlave = null;
            }
        }

        public override void SendLinkLayerTestFunction()
        {
        }

        public void SetLinkLayerStateChanged(LinkLayerStateChanged callback, object parameter)
        {
            stateChanged = callback;
            stateChangedParameter = parameter;
        }
    }
}

