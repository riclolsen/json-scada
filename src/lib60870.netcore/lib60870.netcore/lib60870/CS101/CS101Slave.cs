/*
 *  CS101Slave.cs
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
using System.Collections.Generic;
using lib60870;
using lib60870.linklayer;
using System.IO;

namespace lib60870
{
    namespace CS101
    {

        /// <summary>
        /// CS 101 slave implementation (implements Slave interface)
        /// </summary>
        public class CS101Slave : Slave, ISecondaryApplicationLayer, IMasterConnection
        {

            private void DebugLog(string msg)
            {
                if (debugOutput)
                {
                    Console.Write("CS101 SLAVE: ");
                    Console.WriteLine(msg);
                }
            }
				
            /********************************************
			 * IASDUSender
			 ********************************************/

            void IMasterConnection.SendASDU(ASDU asdu)
            {
                SendASDU(asdu);
            }

            void IMasterConnection.SendACT_CON(ASDU asdu, bool negative)
            {
                asdu.Cot = CauseOfTransmission.ACTIVATION_CON;
                asdu.IsNegative = negative;

                SendASDU(asdu);
            }

            void IMasterConnection.SendACT_TERM(ASDU asdu)
            {
                asdu.Cot = CauseOfTransmission.ACTIVATION_TERMINATION;
                asdu.IsNegative = false;

                SendASDU(asdu);
            }

            ApplicationLayerParameters IMasterConnection.GetApplicationLayerParameters()
            {
                return parameters;
            }

            /********************************************
			 * ISecondaryApplicationLayer
			 ********************************************/

            bool ISecondaryApplicationLayer.IsClass1DataAvailable()
            {
                return IsUserDataClass1Available();
            }

            BufferFrame ISecondaryApplicationLayer.GetClass1Data()
            {
                return DequeueUserDataClass1();
            }

            BufferFrame ISecondaryApplicationLayer.GetCLass2Data()
            {
                BufferFrame asdu = DequeueUserDataClass2();

                if (asdu == null)
                    asdu = DequeueUserDataClass1();

                return asdu;
            }

            bool ISecondaryApplicationLayer.HandleReceivedData(byte[] msg, bool isBroadcast, int userDataStart, int userDataLength)
            {
                return HandleApplicationLayer(0, msg, userDataStart, userDataLength);
            }

            void ISecondaryApplicationLayer.ResetCUReceived(bool onlyFcb)
            {
                //TODO delete data queues
                lock (userDataClass1Queue)
                {
                    userDataClass1Queue.Clear();
                }
                lock (userDataClass2Queue)
                {
                    userDataClass2Queue.Clear();
                }
            }

            /********************************************
			 * END ISecondaryApplicationLayer
			 ********************************************/

            //			private bool sendLinkLayerTestFunction = false;

            private LinkLayer linkLayer = null;

            private byte[] buffer = new byte[300];
            private SerialPort port = null;
            private bool running = false;
            private LinkLayerParameters linkLayerParameters;
            private LinkLayerMode linkLayerMode = LinkLayerMode.UNBALANCED;

            PrimaryLinkLayerBalanced primaryLinkLayerBalanced = null;

            private int linkLayerAddress = 0;
            private int linkLayerAddressOtherStation;
            // link layer address of other station in balanced mode

            private Queue<BufferFrame> userDataClass1Queue = new Queue<BufferFrame>();
            private int userDataClass1QueueMaxSize = 100;

            private Queue<BufferFrame> userDataClass2Queue = new Queue<BufferFrame>();
            private int userDataClass2QueueMaxSize = 100;

            private SerialTransceiverFT12 transceiver;

            private FileServer fileServer;

            private bool initialized;

            private ApplicationLayerParameters parameters = new ApplicationLayerParameters();

            /// <summary>
            /// Gets or sets the application layer parameters-
            /// </summary>
            /// <remarks>Should be set before starting the communication</remarks>
            /// <value>application layer parameters.</value>
            public ApplicationLayerParameters Parameters
            {
                get
                {
                    return this.parameters;
                }
                set
                {
                    parameters = value;
                }
            }

            /// <summary>
            /// Gets or sets the direction bit value used for balanced mode (default is false)
            /// </summary>
            /// <value><c>true</c> if DIR is set otherwise, <c>false</c>.</value>
            public bool DIR
            {
                get
                {
                    return linkLayer.DIR;
                }
                set
                {
                    linkLayer.DIR = value;
                }
            }

            /// <summary>
            /// Gets or sets the link layer mode (balanced or unbalanced).
            /// </summary>
            /// <value>The link layer mode.</value>
            public LinkLayerMode LinkLayerMode
            {
                get
                {
                    return this.linkLayerMode;
                }
                set
                {
                    if (initialized == false)
                        linkLayerMode = value;
                }
            }

            /// <summary>
            /// Stops the receive message loop
            /// </summary>
            public void Stop()
            {
                running = false;
            }


            internal bool IsUserDataClass1Available()
            {
                lock (userDataClass1Queue)
                {
                    if (userDataClass1Queue.Count > 0)
                        return true;
                    else
                        return false;
                }
            }

            /// <summary>
            /// Sets the user data queue sizes. When the maximum size is reached the oldest value will be deleted when
            /// a new ASDU is added
            /// </summary>
            /// <param name="class1QueueSize">Class 1 queue size.</param>
            /// <param name="class2QueueSize">Class 2 queue size.</param>
            public void SetUserDataQueueSizes(int class1QueueSize, int class2QueueSize)
            {
                userDataClass1QueueMaxSize = class1QueueSize;
                userDataClass2QueueMaxSize = class2QueueSize;
            }

            /// <summary>
            /// Determines whether the user data class 1 queue is full.
            /// </summary>
            /// <returns><c>true</c> if the queue  is full; otherwise, <c>false</c>.</returns>
            public bool IsUserDataClass1QueueFull()
            {
                return (userDataClass1Queue.Count == userDataClass1QueueMaxSize);
            }

            /// <summary>
            /// Enqueues an ASDU into the class 1 queue (for events, command responses, and other high-priority messages).
            /// </summary>
            /// <param name="asdu">ASDU to enqueue</param>
            public void EnqueueUserDataClass1(ASDU asdu)
            {
                lock (userDataClass1Queue)
                {
                    BufferFrame frame = new BufferFrame(new byte[256], 0);

                    asdu.Encode(frame, parameters);

                    userDataClass1Queue.Enqueue(frame);

                    while (userDataClass1Queue.Count > userDataClass1QueueMaxSize)
                        userDataClass1Queue.Dequeue();
                }
            }

            internal BufferFrame DequeueUserDataClass1()
            {
                lock (userDataClass1Queue)
                {

                    if (userDataClass1Queue.Count > 0)
                        return userDataClass1Queue.Dequeue();
                    else
                        return null;
                }
            }

            internal bool IsUserDataClass2Available()
            {
                lock (userDataClass2Queue)
                {
                    if (userDataClass2Queue.Count > 0)
                        return true;
                    else
                        return false;
                }
            }

            /// <summary>
            /// Determines whether the user data class 2 queue is full.
            /// </summary>
            /// <returns><c>true</c> if the queue  is full; otherwise, <c>false</c>.</returns>
            public bool IsUserDataClass2QueueFull()
            {
                return (userDataClass2Queue.Count == userDataClass2QueueMaxSize);
            }

            /// <summary>
            /// Enqueues an ASDU into the class 2 queue (for periodic measurments, background scan, and other low-priority data).
            /// </summary>
            /// <param name="asdu">ASDU to enqueue</param>
            public void EnqueueUserDataClass2(ASDU asdu)
            {
                lock (userDataClass2Queue)
                {

                    BufferFrame frame = new BufferFrame(new byte[256], 0);

                    asdu.Encode(frame, parameters);

                    userDataClass2Queue.Enqueue(frame);

                    while (userDataClass2Queue.Count > userDataClass2QueueMaxSize)
                        userDataClass2Queue.Dequeue();
                }
            }

            internal BufferFrame DequeueUserDataClass2()
            {
                lock (userDataClass2Queue)
                {

                    if (userDataClass2Queue.Count > 0)
                        return userDataClass2Queue.Dequeue();
                    else
                        return null;
                }
            }

            /// <summary>
            /// Gets or sets the own link layer address.
            /// </summary>
            /// <value>own link layer address.</value>
            public int LinkLayerAddress
            {
                get
                {
                    return this.linkLayerAddress;
                }
                set
                {
                    linkLayerAddress = value;
                }
            }

            /// <summary>
            /// Gets or sets the link layer address of the other station (for balanced mode).
            /// </summary>
            /// <value>link layer address of other station.</value>
            public int LinkLayerAddressOtherStation
            {
                get
                {
                    return this.linkLayerAddressOtherStation;
                }
                set
                {
                    linkLayerAddressOtherStation = value;
                    if (primaryLinkLayerBalanced != null)
                        primaryLinkLayerBalanced.LinkLayerAddressOtherStation = value;
                }
            }

            /// <summary>
            /// Initializes a new instance of the <see cref="lib60870.CS101.CS101Slave"/> class.
            /// </summary>
            /// <param name="port">serial port instance</param>
            /// <param name="parameters">link layer parameters</param>
            public CS101Slave(SerialPort port, LinkLayerParameters parameters = null)
            {
                this.port = port;

                linkLayerParameters = parameters;

                if (linkLayerParameters == null)
                    linkLayerParameters = new LinkLayerParameters();

                transceiver = new SerialTransceiverFT12(port, linkLayerParameters, DebugLog);

                initialized = false;

                fileServer = new FileServer(this, GetAvailableFiles(), DebugLog);
            }

            /// <summary>
            /// Initializes a new instance of the <see cref="lib60870.CS101.CS101Slave"/> class.
            /// </summary>
            /// <param name="serialStream">A stream instance (e.g. TcpClientVirtualSerialPort or TcpServerVirtualSerialPort.</param>
            /// <param name="parameters">link layer parameters</param>
            public CS101Slave(Stream serialStream, LinkLayerParameters parameters = null)
            {
                linkLayerParameters = parameters;

                if (linkLayerParameters == null)
                    linkLayerParameters = new LinkLayerParameters();

                transceiver = new SerialTransceiverFT12(serialStream, linkLayerParameters, DebugLog);

                initialized = false;

                fileServer = new FileServer(this, GetAvailableFiles(), DebugLog);
            }

            internal void SendASDU(ASDU asdu)
            {
                EnqueueUserDataClass1(asdu);
            }


            private bool HandleApplicationLayer(int address, byte[] msg, int userDataStart, int userDataLength)
            {

                ASDU asdu;

                try
                {
                    asdu = new ASDU(parameters, buffer, userDataStart, userDataStart + userDataLength);
                }
                catch (ASDUParsingException e)
                {
                    DebugLog("ASDU parsing failed: " + e.Message);
                    return false;
                }

                bool messageHandled = false;

                switch (asdu.TypeId)
                {

                    case TypeID.C_IC_NA_1: /* 100 - interrogation command */

                        DebugLog("Rcvd interrogation command C_IC_NA_1");

                        if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.DEACTIVATION))
                        {
                            if (this.interrogationHandler != null)
                            {

                                InterrogationCommand irc = (InterrogationCommand)asdu.GetElement(0);

                                if (this.interrogationHandler(this.InterrogationHandlerParameter, this, asdu, irc.QOI))
                                    messageHandled = true;
                            }
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }

                        break;

                    case TypeID.C_CI_NA_1: /* 101 - counter interrogation command */

                        DebugLog("Rcvd counter interrogation command C_CI_NA_1");

                        if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.DEACTIVATION))
                        {
                            if (this.counterInterrogationHandler != null)
                            {

                                CounterInterrogationCommand cic = (CounterInterrogationCommand)asdu.GetElement(0);

                                if (this.counterInterrogationHandler(this.counterInterrogationHandlerParameter, this, asdu, cic.QCC))
                                    messageHandled = true;
                            }
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }

                        break;

                    case TypeID.C_RD_NA_1: /* 102 - read command */

                        DebugLog("Rcvd read command C_RD_NA_1");

                        if (asdu.Cot == CauseOfTransmission.REQUEST)
                        {

                            DebugLog("Read request for object: " + asdu.Ca);

                            if (this.readHandler != null)
                            {
                                ReadCommand rc = (ReadCommand)asdu.GetElement(0);

                                if (this.readHandler(this.readHandlerParameter, this, asdu, rc.ObjectAddress))
                                    messageHandled = true;

                            }

                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }

                        break;

                    case TypeID.C_CS_NA_1: /* 103 - Clock synchronization command */

                        DebugLog("Rcvd clock sync command C_CS_NA_1");

                        if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                        {

                            if (this.clockSynchronizationHandler != null)
                            {

                                ClockSynchronizationCommand csc = (ClockSynchronizationCommand)asdu.GetElement(0);

                                if (this.clockSynchronizationHandler(this.clockSynchronizationHandlerParameter,
                                    this, asdu, csc.NewTime))
                                    messageHandled = true;
                            }

                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }

                        break;

                    case TypeID.C_TS_NA_1: /* 104 - test command */

                        DebugLog("Rcvd test command C_TS_NA_1");

                        if (asdu.Cot != CauseOfTransmission.ACTIVATION)
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                        }
                        else
                            asdu.Cot = CauseOfTransmission.ACTIVATION_CON;

                        this.SendASDU(asdu);

                        messageHandled = true;

                        break;

                    case TypeID.C_RP_NA_1: /* 105 - Reset process command */

                        DebugLog("Rcvd reset process command C_RP_NA_1");

                        if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                        {

                            if (this.resetProcessHandler != null)
                            {

                                ResetProcessCommand rpc = (ResetProcessCommand)asdu.GetElement(0);

                                if (this.resetProcessHandler(this.resetProcessHandlerParameter,
                                    this, asdu, rpc.QRP))
                                    messageHandled = true;
                            }

                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }


                        break;

                    case TypeID.C_CD_NA_1: /* 106 - Delay acquisition command */

                        DebugLog("Rcvd delay acquisition command C_CD_NA_1");

                        if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.SPONTANEOUS))
                        {
                            if (this.delayAcquisitionHandler != null)
                            {

                                DelayAcquisitionCommand dac = (DelayAcquisitionCommand)asdu.GetElement(0);

                                if (this.delayAcquisitionHandler(this.delayAcquisitionHandlerParameter,
                                    this, asdu, dac.Delay))
                                    messageHandled = true;
                            }
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            this.SendASDU(asdu);
                        }

                        break;

                }

                if (messageHandled == false)
                    messageHandled = fileServer.HandleFileAsdu(asdu);

                if ((messageHandled == false) && (this.asduHandler != null))
                if (this.asduHandler(this.asduHandlerParameter, this, asdu))
                    messageHandled = true;

                if (messageHandled == false)
                {
                    asdu.Cot = CauseOfTransmission.UNKNOWN_TYPE_ID;
                    asdu.IsNegative = true;
                    this.SendASDU(asdu);
                }

                return true;
            }

            private BufferFrame GetUserData()
            {
                if (IsUserDataClass1Available())
                    return DequeueUserDataClass1();
                else if (IsUserDataClass2Available())
                    return DequeueUserDataClass2();
                else
                    return null;

            }

            /// <summary>
            /// Sends a link layer test function.
            /// </summary>
            public void SendLinkLayerTestFunction()
            {
                linkLayer.SendTestFunction();
            }

            /// <summary>
            /// Run a the message receiver and state machines once. Can be used if no threads should be used.
            /// </summary>
            public void Run()
            {
                if (initialized == false)
                {
				
                    linkLayer = new LinkLayer(buffer, linkLayerParameters, transceiver, DebugLog);
                    linkLayer.LinkLayerMode = linkLayerMode;

                    if (linkLayerMode == LinkLayerMode.BALANCED)
                    {

                        PrimaryLinkLayerBalanced primaryLinkLayerBalanced = new PrimaryLinkLayerBalanced(linkLayer, GetUserData, DebugLog);
                        primaryLinkLayerBalanced.LinkLayerAddressOtherStation = linkLayerAddressOtherStation;

                        linkLayer.SetPrimaryLinkLayer(primaryLinkLayerBalanced);

                        linkLayer.SetSecondaryLinkLayer(new SecondaryLinkLayerBalanced(linkLayer, linkLayerAddressOtherStation, HandleApplicationLayer, DebugLog));
                    }
                    else
                    {
                        linkLayer.SetSecondaryLinkLayer(new SecondaryLinkLayerUnbalanced(linkLayer, linkLayerAddress, this, DebugLog));
                    }

                    initialized = true;
                }

                if (fileServer != null)
                    fileServer.HandleFileTransmission();

                linkLayer.Run();
            }


            /// <summary>
            /// Starts a loop that handles incoming messages. 
            /// </summary>
            /// It is best to be started in a separate thread.
            /// The loop can be stopped with the Stop method.
            public void ReceiveMessageLoop()
            {
                running = true;

                if (port != null)
                {
                    if (port.IsOpen == false)
                        port.Open();

                    port.DiscardInBuffer();
                }

                while (running)
                {
                    Run();
                }

                if (port != null)
                    port.Close();
            }

            public override int FileTimeout 
            {
                get {
                    if (fileServer != null)
                        return (int)fileServer.Timeout;
                    else
                        return 0;
                }

                set {
                    if (fileServer != null)
                        fileServer.Timeout = value;
                }
            }

        }

    }
}
