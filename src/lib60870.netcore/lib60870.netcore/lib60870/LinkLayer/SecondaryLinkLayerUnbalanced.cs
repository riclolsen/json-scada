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
    internal class SecondaryLinkLayerUnbalanced : SecondaryLinkLayer
    {
        // expected value of next frame count bit (FCB)
        private bool expectedFcb = true;

        private Action<string> DebugLog;
        private LinkLayer linkLayer;
        private ISecondaryApplicationLayer applicationLayer;

        private int linkLayerAddress = 0;

        public SecondaryLinkLayerUnbalanced(LinkLayer linkLayer, int address, ISecondaryApplicationLayer applicationLayer, Action<string> debugLog)
        {
            this.linkLayer = linkLayer;
            this.linkLayerAddress = address;
            this.DebugLog = debugLog;
            this.applicationLayer = applicationLayer;
        }

        public override int Address
        {
            get { return linkLayerAddress; }
            set { linkLayerAddress = value; }
        }

        private bool CheckFCB(bool fcb)
        {
            if (fcb != expectedFcb)
            {
                DebugLog("SLL - ERROR: Frame count bit (FCB) invalid!");
                //TODO change link status
                return false;
            }
            else
            {
                expectedFcb = !expectedFcb;
                return true;
            }
        }

        public override void HandleMessage(FunctionCodePrimary fcp, bool isBroadcast, int address, bool fcb, bool fcv, byte[] msg, int userDataStart, int userDataLength)
        {
            // check frame count bit if fcv == true
            if (fcv)
            {
                if (CheckFCB(fcb) == false)
                    return;
            }
				
            switch (fcp)
            {

                case FunctionCodePrimary.REQUEST_LINK_STATUS:
                    DebugLog("SLL - REQUEST LINK STATUS");
                    {
                        bool accessDemand = applicationLayer.IsClass1DataAvailable();

                        linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.STATUS_OF_LINK_OR_ACCESS_DEMAND, linkLayerAddress, accessDemand, false);
                    }
                    break;

                case FunctionCodePrimary.RESET_REMOTE_LINK:
                    DebugLog("SLL - RESET REMOTE LINK");
                    {
                        expectedFcb = true;

                        if (linkLayer.linkLayerParameters.UseSingleCharACK)
                            linkLayer.SendSingleCharACK();
                        else
                            linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.ACK, linkLayerAddress, false, false);

                        applicationLayer.ResetCUReceived(false);
                    }

                    break;

                case FunctionCodePrimary.RESET_FCB:
                    DebugLog("SLL - RESET FCB");
                    {
                        expectedFcb = true;

                        if (linkLayer.linkLayerParameters.UseSingleCharACK)
                            linkLayer.SendSingleCharACK();
                        else
                            linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.ACK, linkLayerAddress, false, false);

                        applicationLayer.ResetCUReceived(true);
                    }
                    break;

                case FunctionCodePrimary.REQUEST_USER_DATA_CLASS_2:
                    DebugLog("SLL - REQUEST USER DATA CLASS 2");
                    {
                        BufferFrame asdu = applicationLayer.GetCLass2Data();

                        bool accessDemand = applicationLayer.IsClass1DataAvailable();

                        if (asdu != null)
                            linkLayer.SendVariableLengthFrameSecondary(FunctionCodeSecondary.RESP_USER_DATA, linkLayerAddress, accessDemand, false, asdu);
                        else
                        {
                            if (linkLayer.linkLayerParameters.UseSingleCharACK && (accessDemand == false))
                                linkLayer.SendSingleCharACK();
                            else
                                linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.RESP_NACK_NO_DATA, linkLayerAddress, accessDemand, false);
                        }

                    }
                    break;

                case FunctionCodePrimary.REQUEST_USER_DATA_CLASS_1:
                    DebugLog("SLL - REQUEST USER DATA CLASS 1");
                    {
                        BufferFrame asdu = applicationLayer.GetClass1Data();

                        bool accessDemand = applicationLayer.IsClass1DataAvailable();

                        if (asdu != null)
                            linkLayer.SendVariableLengthFrameSecondary(FunctionCodeSecondary.RESP_USER_DATA, linkLayerAddress, accessDemand, false, asdu);
                        else
                        {
                            if (linkLayer.linkLayerParameters.UseSingleCharACK && (accessDemand == false))
                                linkLayer.SendSingleCharACK();
                            else
                                linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.RESP_NACK_NO_DATA, linkLayerAddress, accessDemand, false);
                        }

                    }
                    break;

                case FunctionCodePrimary.USER_DATA_CONFIRMED:
                    DebugLog("SLL - USER DATA CONFIRMED");
                    if (userDataLength > 0)
                    {
                        if (applicationLayer.HandleReceivedData(msg, isBroadcast, userDataStart, userDataLength))
                        {

                            bool accessDemand = applicationLayer.IsClass1DataAvailable();

                            linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.ACK, linkLayerAddress, accessDemand, false);
                        }
                    }
                    break;

                case FunctionCodePrimary.USER_DATA_NO_REPLY:
                    DebugLog("SLL - USER DATA NO REPLY");
                    if (userDataLength > 0)
                    {
                        applicationLayer.HandleReceivedData(msg, isBroadcast, userDataStart, userDataLength);
                    }
                    break;

                default:
                    DebugLog("SLL - UNEXPECTED LINK LAYER MESSAGE");
                    linkLayer.SendFixedFrameSecondary(FunctionCodeSecondary.LINK_SERVICE_NOT_IMPLEMENTED, linkLayerAddress, false, false);
                    break;
            }
        }

        public override void RunStateMachine()
        {

        }
    }
}

