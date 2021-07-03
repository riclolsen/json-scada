/*
  *  Copyright 2018 MZ Automation GmbH
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

using lib60870.linklayer;
using System.Threading;
using System.IO;

namespace lib60870.CS101
{
    public class CS101Master : Master, IPrimaryLinkLayerCallbacks
    {
        protected Thread workerThread = null;

        internal LinkLayer linkLayer = null;

        internal FileClient fileClient = null;

        protected SerialPort port = null;
        protected bool running = false;

        private bool fatalError = false;

        private void ReceiveMessageLoop()
        {
            running = true;

            while (running)
            {
                Run();

                Thread.Sleep(1);
            }
        }

        /// <summary>
        /// Value of DIR bit when sending messages.
        /// </summary>
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
        /// Run the protocol state machines a single time.
        /// Alternative to Start/Stop when no background thread should be used
        /// Has to be called frequently
        /// </summary>
        public void Run()
        {
            if(fatalError == false)
            {
                linkLayer.Run();

                if (fileClient != null)
                    fileClient.HandleFileService();
            }
                        
        }

        private void fatalErrorHandler(object sender, EventArgs eventArgs)
        {
            fatalError = true;
        }

        public void AddPortDeniedHandler(EventHandler eventHandler)
        {
            linkLayer.AddPortDeniedHandler(eventHandler);
        }

        /// <summary>
        /// Start a background thread running the master
        /// </summary>
        public void Start()
        {
            linkLayer.AddPortDeniedHandler(fatalErrorHandler);
            
            if (port != null)
            {
                if (port.IsOpen == false)
                    port.Open();

                port.DiscardInBuffer();
            }

            workerThread = new Thread(ReceiveMessageLoop);

            workerThread.Start();
        }

        /// <summary>
        /// Stop the background thread
        /// </summary>
        public void Stop()
        {
            if (running)
            {
                running = false;

                if (workerThread != null)
                    workerThread.Join();
            }
        }

        public int OwnAddress
        {
            get
            {
                return linkLayer.OwnAddress;
            }
            set
            {
                linkLayer.OwnAddress = value;
            }
        }

        public LinkLayerState GetLinkLayerState()
        {
            if (linkLayer.LinkLayerMode == LinkLayerMode.BALANCED)
                return primaryLinkLayer.GetLinkLayerState();
            else
                return linkLayerUnbalanced.GetStateOfSlave(slaveAddress);
        }

        public override void SetReceivedRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            linkLayer.SetReceivedRawMessageHandler(handler, parameter);
        }

        public override void SetSentRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            linkLayer.SetSentRawMessageHandler(handler, parameter);
        }

        private PrimaryLinkLayerUnbalanced linkLayerUnbalanced = null;
        private PrimaryLinkLayerBalanced primaryLinkLayer = null;

        private SerialTransceiverFT12 transceiver;

        /* selected slave address for unbalanced mode */
        private int slaveAddress = 0;

        /* buffer to read data from serial line */
        private byte[] buffer = new byte[300];

        private LinkLayerParameters linkLayerParameters;
        private ApplicationLayerParameters appLayerParameters;

        private ASDUReceivedHandler asduReceivedHandler = null;
        private object asduReceivedHandlerParameter = null;

        /* user data queue for balanced mode */
        private Queue<BufferFrame> userDataQueue;

        private void DebugLog(string msg)
        {
            if (debugOutput)
            {
                Console.Write("CS101 MASTER: ");
                Console.WriteLine(msg);
            }
        }

        public CS101Master(SerialPort port, LinkLayerMode mode, LinkLayerParameters llParams = null, ApplicationLayerParameters alParams = null)
        {
            if (llParams == null)
                this.linkLayerParameters = new LinkLayerParameters();
            else
                this.linkLayerParameters = llParams;

            if (alParams == null)
                this.appLayerParameters = new ApplicationLayerParameters();
            else
                this.appLayerParameters = alParams;


            this.transceiver = new SerialTransceiverFT12(port, linkLayerParameters, DebugLog);

            linkLayer = new LinkLayer(buffer, linkLayerParameters, transceiver, DebugLog);
            linkLayer.LinkLayerMode = mode;

            if (mode == LinkLayerMode.BALANCED)
            {
                linkLayer.DIR = true;

                primaryLinkLayer = new PrimaryLinkLayerBalanced(linkLayer, GetUserData, DebugLog);

                linkLayer.SetPrimaryLinkLayer(primaryLinkLayer);
                linkLayer.SetSecondaryLinkLayer(new SecondaryLinkLayerBalanced(linkLayer, 0, HandleApplicationLayer, DebugLog));

                userDataQueue = new Queue<BufferFrame>();
            }
            else
            {
                linkLayerUnbalanced = new PrimaryLinkLayerUnbalanced(linkLayer, this, DebugLog);
                linkLayer.SetPrimaryLinkLayer(linkLayerUnbalanced);
            }

            this.port = port;

            this.fileClient = null;
        }

        public CS101Master(Stream serialStream, LinkLayerMode mode, LinkLayerParameters llParams = null, ApplicationLayerParameters alParams = null)
        {
            if (llParams == null)
                this.linkLayerParameters = new LinkLayerParameters();
            else
                this.linkLayerParameters = llParams;

            if (alParams == null)
                this.appLayerParameters = new ApplicationLayerParameters();
            else
                this.appLayerParameters = alParams;


            this.transceiver = new SerialTransceiverFT12(serialStream, linkLayerParameters, DebugLog);

            linkLayer = new LinkLayer(buffer, linkLayerParameters, transceiver, DebugLog);
            linkLayer.LinkLayerMode = mode;

            if (mode == LinkLayerMode.BALANCED)
            {
                linkLayer.DIR = true;

                primaryLinkLayer = new PrimaryLinkLayerBalanced(linkLayer, GetUserData, DebugLog);

                linkLayer.SetPrimaryLinkLayer(primaryLinkLayer);
                linkLayer.SetSecondaryLinkLayer(new SecondaryLinkLayerBalanced(linkLayer, 0, HandleApplicationLayer, DebugLog));

                userDataQueue = new Queue<BufferFrame>();
            }
            else
            {
                linkLayerUnbalanced = new PrimaryLinkLayerUnbalanced(linkLayer, this, DebugLog);
                linkLayer.SetPrimaryLinkLayer(linkLayerUnbalanced);
            }

            this.fileClient = null;
        }

        /// <summary>
        /// Sets the timeouts for receiving messages (in milliseconds)
        /// </summary>
        /// <param name="messageTimeout">Timeout to wait for the first character of a message</param>
        /// <param name="characterTimeout">Timeout to wait for next characters in a message</param>
        public void SetTimeouts(int messageTimeout, int characterTimeout)
        {
            this.transceiver.SetTimeouts(messageTimeout, characterTimeout);
        }


        public void SetASDUReceivedHandler(ASDUReceivedHandler handler, object parameter)
        {
            asduReceivedHandler = handler;
            asduReceivedHandlerParameter = parameter;
        }

        public void AddSlave(int slaveAddress)
        {
            linkLayerUnbalanced.AddSlaveConnection(slaveAddress);
        }

        public LinkLayerState GetLinkLayerState(int slaveAddress)
        {
            if (linkLayerUnbalanced != null)
                return linkLayerUnbalanced.GetStateOfSlave(slaveAddress);
            else
                return primaryLinkLayer.GetLinkLayerState();
        }

        public void SetLinkLayerStateChangedHandler(LinkLayerStateChanged handler, object parameter)
        {
            if (linkLayerUnbalanced != null)
                linkLayerUnbalanced.SetLinkLayerStateChanged(handler, parameter);
            else
                primaryLinkLayer.SetLinkLayerStateChanged(handler, parameter);
        }

        /// <summary>
        /// Gets or sets the link layer slave address
        /// </summary>
        /// <value>Slave link layer address.</value>
        public int SlaveAddress
        {
            set
            {
                UseSlaveAddress(value);
            }

            get
            {
                if (primaryLinkLayer == null)
                    return this.slaveAddress;
                else
                    return primaryLinkLayer.LinkLayerAddressOtherStation;
            }
        }

        /// <summary>
        /// Sets the slave link layer address to be used
        /// </summary>
        /// <param name="slaveAddress">Slave link layer address.</param>
        public void UseSlaveAddress(int slaveAddress)
        {
            if (primaryLinkLayer != null)
                primaryLinkLayer.LinkLayerAddressOtherStation = slaveAddress;
            else
                this.slaveAddress = slaveAddress;
        }

        void IPrimaryLinkLayerCallbacks.AccessDemand(int slaveAddress)
        {
            DebugLog("Access demand slave " + slaveAddress);
            linkLayerUnbalanced.RequestClass1Data(slaveAddress);
        }

        void IPrimaryLinkLayerCallbacks.UserData(int slaveAddress, byte[] message, int start, int length)
        {
            DebugLog("User data slave " + slaveAddress);

            ASDU asdu;

            try
            {
                asdu = new ASDU(appLayerParameters, message, start, start + length);
            }
            catch (ASDUParsingException e)
            {
                DebugLog("ASDU parsing failed: " + e.Message);
                return;
            }

            bool messageHandled = false;

            if (fileClient != null)
                messageHandled = fileClient.HandleFileAsdu(asdu);

            if (messageHandled == false)
            {
                if (asduReceivedHandler != null)
                    asduReceivedHandler(asduReceivedHandlerParameter, slaveAddress, asdu);
            }

        }

        void IPrimaryLinkLayerCallbacks.Timeout(int slaveAddress)
        {
            DebugLog("Timeout accessing slave " + slaveAddress);
        }

        public void PollSingleSlave(int address)
        {
            try
            {
                if (linkLayerUnbalanced != null)
                    linkLayerUnbalanced.RequestClass2Data(address);
            }
            catch (LinkLayerBusyException)
            {
                DebugLog("Link layer busy");
            }
        }

        public void RequestClass1Data(int address)
        {
            try
            {
                if (linkLayerUnbalanced != null)
                     linkLayerUnbalanced.RequestClass1Data(address);
            }
            catch (LinkLayerBusyException)
            {
                DebugLog("Link layer busy");
            }
        }

        private void EnqueueUserData(ASDU asdu)
        {
            if (linkLayerUnbalanced != null)
            {
                //TODO problem -> buffer frame needs own buffer so that the message can be stored.
                BufferFrame frame = new BufferFrame(buffer, 0);

                asdu.Encode(frame, appLayerParameters);

                linkLayerUnbalanced.SendConfirmed(slaveAddress, frame);
            }
            else
            {
                lock (userDataQueue)
                {

                    BufferFrame frame = new BufferFrame(new byte[256], 0);

                    asdu.Encode(frame, appLayerParameters);

                    userDataQueue.Enqueue(frame);
                }
            }
        }

        private BufferFrame DequeueUserData()
        {
            lock (userDataQueue)
            {

                if (userDataQueue.Count > 0)
                    return userDataQueue.Dequeue();
                else
                    return null;
            }
        }

        private bool IsUserDataAvailable()
        {
            lock (userDataQueue)
            {
                if (userDataQueue.Count > 0)
                    return true;
                else
                    return false;
            }
        }

        /// <summary>
        /// Callback function forPrimaryLinkLayerBalanced
        /// </summary>
        /// <returns>The next ASDU to send</returns>
        private BufferFrame GetUserData()
        {
            BufferFrame asdu = null;

            if (IsUserDataAvailable())
                return DequeueUserData();

            return asdu;
        }

        /// <summary>
        /// Callback function for secondary link layer (balanced mode)
        /// </summary>
        private bool HandleApplicationLayer(int address, byte[] msg, int userDataStart, int userDataLength)
        {

            ASDU asdu;

            try
            {
                asdu = new ASDU(appLayerParameters, buffer, userDataStart, userDataStart + userDataLength);
            }
            catch (ASDUParsingException e)
            {
                DebugLog("ASDU parsing failed: " + e.Message);
                return false;
            }

            bool messageHandled = false;

            if (fileClient != null)
                messageHandled = fileClient.HandleFileAsdu(asdu);

            if (messageHandled == false)
            {
                if (asduReceivedHandler != null)
                    messageHandled = asduReceivedHandler(asduReceivedHandlerParameter, address, asdu);
            }

            return messageHandled;
        }


        public void SendLinkLayerTestFunction()
        {
            linkLayer.SendTestFunction();
        }

        public override void SendInterrogationCommand(CauseOfTransmission cot, int ca, byte qoi)
        {
            ASDU asdu = new ASDU(appLayerParameters, cot, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new InterrogationCommand(0, qoi));

            EnqueueUserData(asdu);
        }

        public override void SendCounterInterrogationCommand(CauseOfTransmission cot, int ca, byte qcc)
        {
            ASDU asdu = new ASDU(appLayerParameters, cot, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new CounterInterrogationCommand(0, qcc));

            EnqueueUserData(asdu);
        }

        public override void SendReadCommand(int ca, int ioa)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.REQUEST, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new ReadCommand(ioa));

            EnqueueUserData(asdu);
        }

        public override void SendClockSyncCommand(int ca, CP56Time2a time)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new ClockSynchronizationCommand(0, time));

            EnqueueUserData(asdu);
        }

        public override void SendTestCommand(int ca)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new TestCommand());

            EnqueueUserData(asdu);
        }

        public override void SendTestCommandWithCP56Time2a(int ca, ushort tsc, CP56Time2a time)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new TestCommandWithCP56Time2a(tsc, time));

            EnqueueUserData(asdu);
        }

        public override void SendResetProcessCommand(CauseOfTransmission cot, int ca, byte qrp)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new ResetProcessCommand(0, qrp));

            EnqueueUserData(asdu);
        }

        public override void SendDelayAcquisitionCommand(CauseOfTransmission cot, int ca, CP16Time2a delay)
        {
            ASDU asdu = new ASDU(appLayerParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)appLayerParameters.OA, ca, false);

            asdu.AddInformationObject(new DelayAcquisitionCommand(0, delay));

            EnqueueUserData(asdu);
        }

        public override void SendControlCommand(CauseOfTransmission cot, int ca, InformationObject sc)
        {
            ASDU controlCommand = new ASDU(appLayerParameters, cot, false, false, (byte)appLayerParameters.OA, ca, false);

            controlCommand.AddInformationObject(sc);

            EnqueueUserData(controlCommand);
        }

        public override void SendASDU(ASDU asdu)
        {
            EnqueueUserData(asdu);
        }

        public override ApplicationLayerParameters GetApplicationLayerParameters()
        {
            return appLayerParameters;
        }

        public override void GetFile(int ca, int ioa, NameOfFile nof, IFileReceiver receiver)
        {
            if (fileClient == null)
                fileClient = new FileClient(this, DebugLog);

            fileClient.RequestFile(ca, ioa, nof, receiver);
        }

        public override void SendFile (int ca, int ioa, NameOfFile nof, IFileProvider fileProvider)
        {
            if (fileClient == null)
                fileClient = new FileClient (this, DebugLog);

            fileClient.SendFile (ca, ioa, nof, fileProvider);
        }
    }

}
