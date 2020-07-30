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

namespace lib60870.linklayer
{
    internal class PrimaryLinkLayerBalanced : PrimaryLinkLayer
    {
        private Action<string> DebugLog;

        private PrimaryLinkLayerState primaryState = PrimaryLinkLayerState.IDLE;
        private LinkLayerState state = LinkLayerState.IDLE;

        private bool waitingForResponse = false;
        private long lastSendTime;
        private long originalSendTime;
        private bool sendLinkLayerTestFunction = false;
        private bool nextFcb = true;

        private BufferFrame lastSendASDU = null;
        // last send ASDU for message repetition after timeout

        private int linkLayerAddressOtherStation = 0;

        private LinkLayer linkLayer;

        Func<BufferFrame> GetUserData;

        private LinkLayerStateChanged stateChangedCallback = null;
        private object stateChangedCallbackParameter = null;

        public PrimaryLinkLayerBalanced(LinkLayer linkLayer, Func<BufferFrame> getUserData, Action<string> debugLog)
        {
            this.DebugLog = debugLog;
            this.GetUserData = getUserData;
            this.linkLayer = linkLayer;
        }

        public void SetLinkLayerStateChanged(LinkLayerStateChanged handler, object parameter)
        {
            stateChangedCallback = handler;
            stateChangedCallbackParameter = parameter;
        }

        public LinkLayerState GetLinkLayerState()
        {
            return state;
        }

        public int LinkLayerAddressOtherStation
        {
            set
            { 
                linkLayerAddressOtherStation = value; 
            }

            get
            {
                return linkLayerAddressOtherStation;
            }
        }

        private void SetNewState(LinkLayerState newState)
        {
            if (newState != state)
            {
                state = newState;

                if (stateChangedCallback != null)
                    stateChangedCallback(stateChangedCallbackParameter, -1, newState);
            }
        }

        public override void HandleMessage(FunctionCodeSecondary fcs, bool dir, bool dfc, 
                                     int address, byte[] msg, int userDataStart, int userDataLength)
        {
            PrimaryLinkLayerState newState = primaryState;

            if (dfc)
            {

                //TODO stop sending ASDUs; only send Status of link requests

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

                SetNewState(LinkLayerState.BUSY);
                primaryState = newState;
                return;
            }

            switch (fcs)
            {

                case FunctionCodeSecondary.ACK:
				//TODO what to do if we are not waiting for a response?
                    DebugLog("PLL - received ACK");
                    if (primaryState == PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK)
                    {
                        newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        SetNewState(LinkLayerState.AVAILABLE);
                    }
                    else if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                    {

                        if (sendLinkLayerTestFunction)
                            sendLinkLayerTestFunction = false;
						
                        newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        SetNewState(LinkLayerState.AVAILABLE);
                    }

                    waitingForResponse = false;
                    break;

                case FunctionCodeSecondary.NACK:
                    DebugLog("PLL - received NACK");
                    if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                    {
                        newState = PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY;
                        SetNewState(LinkLayerState.BUSY);
                    }
                    break;

                case FunctionCodeSecondary.RESP_USER_DATA:

                    newState = PrimaryLinkLayerState.IDLE;
                    SetNewState(LinkLayerState.ERROR);

                    break;

                case FunctionCodeSecondary.RESP_NACK_NO_DATA:

                    newState = PrimaryLinkLayerState.IDLE;
                    SetNewState(LinkLayerState.ERROR);

                    break;


                case FunctionCodeSecondary.STATUS_OF_LINK_OR_ACCESS_DEMAND:	
                    DebugLog("PLL - received STATUS OF LINK");
                    if (primaryState == PrimaryLinkLayerState.EXECUTE_REQUEST_STATUS_OF_LINK)
                    {
                        DebugLog("PLL - SEND RESET REMOTE LINK to address " + linkLayerAddressOtherStation);
                        linkLayer.SendFixedFramePrimary(FunctionCodePrimary.RESET_REMOTE_LINK, linkLayerAddressOtherStation, false, false);
                        lastSendTime = SystemUtils.currentTimeMillis();
                        waitingForResponse = true;
                        newState = PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK;
                        SetNewState(LinkLayerState.BUSY);
                    }
                    else
                    { /* illegal message */
                        newState = PrimaryLinkLayerState.IDLE;
                        SetNewState(LinkLayerState.ERROR);
                    }

                    break;

                case FunctionCodeSecondary.LINK_SERVICE_NOT_FUNCTIONING:
                case FunctionCodeSecondary.LINK_SERVICE_NOT_IMPLEMENTED:
                    DebugLog("PLL - link layer service not functioning/not implemented in secondary station");

                    if (sendLinkLayerTestFunction)
                        sendLinkLayerTestFunction = false;

                    if (primaryState == PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM)
                    {
                        newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        SetNewState(LinkLayerState.AVAILABLE);
                    }
                    break;

                default:
                    DebugLog("UNEXPECTED SECONDARY LINK LAYER MESSAGE");
                    break;
            }

            DebugLog("PLL RECV - old state: " + primaryState.ToString() + " new state: " + newState.ToString());

            primaryState = newState;

        }

        public override void SendLinkLayerTestFunction()
        {
            sendLinkLayerTestFunction = true;
        }

        public override void RunStateMachine()
        {
            PrimaryLinkLayerState newState = primaryState;

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
                        if (SystemUtils.currentTimeMillis() > (lastSendTime + linkLayer.TimeoutForACK))
                        {
                            linkLayer.SendFixedFramePrimary(FunctionCodePrimary.REQUEST_LINK_STATUS, linkLayerAddressOtherStation, false, false);
                            lastSendTime = SystemUtils.currentTimeMillis();
                        }
                    }
                    else
                    {
                        DebugLog("PLL - SEND RESET REMOTE LINK to address " + linkLayerAddressOtherStation);
                        linkLayer.SendFixedFramePrimary(FunctionCodePrimary.RESET_REMOTE_LINK, linkLayerAddressOtherStation, false, false);
                        lastSendTime = SystemUtils.currentTimeMillis();
                        waitingForResponse = true;
                        newState = PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK; 
                    }

                    break;

                case PrimaryLinkLayerState.EXECUTE_RESET_REMOTE_LINK:

                    if (waitingForResponse)
                    {
                        if (SystemUtils.currentTimeMillis() > (lastSendTime + linkLayer.TimeoutForACK))
                        {
                            waitingForResponse = false;
                            newState = PrimaryLinkLayerState.IDLE;
                            SetNewState(LinkLayerState.ERROR);
                        }
                    }
                    else
                    {
                        newState = PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE;
                        SetNewState(LinkLayerState.AVAILABLE);
                    }

                    break;

                case PrimaryLinkLayerState.LINK_LAYERS_AVAILABLE:

                    if (sendLinkLayerTestFunction)
                    {
                        DebugLog("PLL - SEND TEST LINK");
                        linkLayer.SendFixedFramePrimary(FunctionCodePrimary.TEST_FUNCTION_FOR_LINK, linkLayerAddressOtherStation, nextFcb, true);
                        nextFcb = !nextFcb;
                        lastSendTime = SystemUtils.currentTimeMillis();
                        originalSendTime = lastSendTime;
                        newState = PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM;
                    }
                    else
                    {
                        BufferFrame asdu = GetUserData();

                        if (asdu != null)
                        {

                            linkLayer.SendVariableLengthFramePrimary(FunctionCodePrimary.USER_DATA_CONFIRMED, linkLayerAddressOtherStation, nextFcb, true, asdu);

                            nextFcb = !nextFcb;
                            lastSendASDU = asdu;
                            lastSendTime = SystemUtils.currentTimeMillis();
                            originalSendTime = lastSendTime;
                            waitingForResponse = true;

                            newState = PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM;
                        }
                    }

                    break;

                case PrimaryLinkLayerState.EXECUTE_SERVICE_SEND_CONFIRM:

                    if (SystemUtils.currentTimeMillis() > (lastSendTime + linkLayer.TimeoutForACK))
                    {

                        if (SystemUtils.currentTimeMillis() > (originalSendTime + linkLayer.TimeoutRepeat))
                        {
                            DebugLog("TIMEOUT: ASDU not confirmed after repeated transmission");
                            newState = PrimaryLinkLayerState.IDLE;
                            SetNewState(LinkLayerState.ERROR);
                        }
                        else
                        {
                            DebugLog("TIMEOUT: ASDU not confirmed");

                            if (sendLinkLayerTestFunction)
                            {
                                DebugLog("PLL - REPEAT SEND RESET REMOTE LINK");
                                linkLayer.SendFixedFramePrimary(FunctionCodePrimary.TEST_FUNCTION_FOR_LINK, linkLayerAddressOtherStation, !nextFcb, true);
                            }
                            else
                            {
                                DebugLog("PLL - repeat last ASDU");
                                linkLayer.SendVariableLengthFramePrimary(FunctionCodePrimary.USER_DATA_CONFIRMED, linkLayerAddressOtherStation, !nextFcb, true, lastSendASDU);
                            }

                            lastSendTime = SystemUtils.currentTimeMillis();
                        }
                    }

                    break;

                case PrimaryLinkLayerState.SECONDARY_LINK_LAYER_BUSY:
				//TODO - reject new requests from application layer?
                    break;

            }

            if (primaryState != newState)
                DebugLog("PLL - old state: " + primaryState.ToString() + " new state: " + newState.ToString());

            primaryState = newState;

        }
    }
}

