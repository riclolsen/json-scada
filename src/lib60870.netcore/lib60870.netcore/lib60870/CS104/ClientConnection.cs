/*
 *  Copyright 2016-2025 Michael Zillgith
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

using lib60870.CS101;
using System;
using System.Collections.Concurrent;
using System.IO;
using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Reflection;
using System.Security.Cryptography.X509Certificates;
using System.Threading;

namespace lib60870.CS104
{
    public enum MasterConnectionState
    {
        M_CON_STATE_STOPPED = 0, /* only U frames allowed */
        M_CON_STATE_STARTED = 1, /* U, I, S frames allowed */
        M_CON_STATE_UNCONFIRMED_STOPPED = 2 /* only U, S frames allowed */
    }
    /// <summary>
    /// Represents a client (master) connection
    /// </summary>
    public class ClientConnection : IMasterConnection
    {
        private static int connectionsCounter = 0;

        private int connectionID;

        private void DebugLog(string msg)
        {
            if (debugOutput)
            {
                Console.Write("CS104 SLAVE CONNECTION ");
                Console.Write(connectionID);
                Console.Write(": ");
                Console.WriteLine(msg);
            }
        }

        static byte[] STARTDT_CON_MSG = new byte[] { 0x68, 0x04, 0x0b, 0x00, 0x00, 0x00 };

        static byte[] STOPDT_CON_MSG = new byte[] { 0x68, 0x04, 0x23, 0x00, 0x00, 0x00 };

        static byte[] TESTFR_CON_MSG = new byte[] { 0x68, 0x04, 0x83, 0x00, 0x00, 0x00 };

        static byte[] TESTFR_ACT_MSG = new byte[] { 0x68, 0x04, 0x43, 0x00, 0x00, 0x00 };


        private MasterConnectionState state;
        private int sendCount = 0;
        private int receiveCount = 0;

        /* number of unconfirmed messages received */
        private int unconfirmedReceivedIMessages = 0;

        /* T3 parameter handling */
        private UInt64 nextT3Timeout;

        /* TEST-FR con timeout handling */
        private bool waitingForTestFRcon = false;
        private UInt64 nextTestFRConTimeout = 0;

        /* T2 parameter handling */
        private bool timeoutT2Triggered = false;

        /* timestamp when the last confirmation message was sent */
        private UInt64 lastConfirmationTime = System.UInt64.MaxValue;

        private TlsSecurityInformation tlsSecInfo = null;

        private APCIParameters apciParameters;
        private ApplicationLayerParameters alParameters;

        private Server server;

        private bool allowTestCommand = false;
        private bool allowDelayAcquisition = false;

        private ConcurrentQueue<ASDU> receivedASDUs = null;
        private Thread callbackThread = null;
        private bool callbackThreadRunning = false;

        /* data structure for k-size sent ASDU buffer */
        private struct SentASDU
        {
            /* required to identify message in server (low-priority) queue */
            public long entryTime;

            /* -1 if ASDU is not from low-priority queue */
            public int queueIndex;

            /* timestamp when the message was sent (for T1 timeout) */
            public long sentTime;

            /* sequence number used to send the message */
            public int seqNo;
        }

        private int maxSentASDUs;
        private int oldestSentASDU = -1;
        private int newestSentASDU = -1;
        private SentASDU[] sentASDUs = null;

        private ASDUQueue lowPrioQueue = null;

        private ASDUQueue highPrioQueue = null;

        private FileServer fileServer;

        /// <summary>
        /// Retrieves the ASDU queue used for processing ASDUs in the system. This queue holds the low-priority ASDUs for further handling.
        /// </summary>
        /// <returns>
        /// The ASDU queue containing low-priority ASDUs.
        /// </returns>
        internal ASDUQueue GetASDUQueue()
        {
            return lowPrioQueue;
        }

        /// <summary>
        /// Continuously processes the received ASDUs in a separate callback thread. The method dequeues ASDUs from the queue and handles them using the `HandleASDU` method.
        /// </summary>
        private void ProcessASDUs()
        {
            callbackThreadRunning = true;

            while (callbackThreadRunning)
            {

                try
                {
                    while ((receivedASDUs.Count > 0) && (callbackThreadRunning) && (running))
                    {

                        ASDU asdu;

                        if (receivedASDUs.TryDequeue(out asdu))
                        {
                            HandleASDU(asdu);
                        }

                    }

                    Thread.Sleep(50);
                }
                catch (ASDUParsingException)
                {
                    DebugLog("Failed to parse ASDU --> close connection");
                    running = false;
                }

            }

            DebugLog("ProcessASDUs exit thread");
        }

        private IPEndPoint remoteEndpoint;

        /// <summary>
        /// Gets the remote endpoint (client IP address and TCP port)
        /// </summary>
        /// <value>The remote IP endpoint</value>
        public IPEndPoint RemoteEndpoint
        {
            get
            {
                return remoteEndpoint;
            }
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="ClientConnection"/> class, establishing a connection using the provided socket and configuration details.
        /// </summary>
        /// <param name="socket">The socket used for the connection, representing the network connection with the client.</param>
        /// <param name="tlsSecInfo">The TLS security information used to manage the security of the connection, including certificates and chain validation.</param>
        /// <param name="apciParameters">The parameters related to the Application Protocol Control Information (APCI), such as the sequence numbers and connection settings.</param>
        /// <param name="parameters">The application layer parameters for the communication, including configuration for encoding/decoding data.</param>
        /// <param name="server">The server object that manages the overall communication and handles requests related to files and ASDUs.</param>
        /// <param name="asduQueue">The ASDU queue that holds the low-priority ASDUs for processing in the connection.</param>
        /// <param name="debugOutput">A flag indicating whether debug output should be enabled for logging the connection's activities.</param>
        internal ClientConnection(Socket socket, TlsSecurityInformation tlsSecInfo, APCIParameters apciParameters, ApplicationLayerParameters parameters, Server server, ASDUQueue asduQueue, bool debugOutput)
        {
            state = MasterConnectionState.M_CON_STATE_STOPPED;
            connectionsCounter++;
            connectionID = connectionsCounter;

            remoteEndpoint = (IPEndPoint)socket.RemoteEndPoint;

            this.apciParameters = apciParameters;
            alParameters = parameters;
            this.server = server;
            lowPrioQueue = asduQueue;
            this.debugOutput = debugOutput;

            ResetT3Timeout((UInt64)SystemUtils.currentTimeMillis());

            maxSentASDUs = apciParameters.K;
            sentASDUs = new SentASDU[maxSentASDUs];

            receivedASDUs = new ConcurrentQueue<ASDU>();
            highPrioQueue = new ASDUQueue(server.MaxHighPrioQueueSize, server.EnqueueMode, alParameters, DebugLog);

            socketStream = new NetworkStream(socket);
            this.socket = socket;
            this.tlsSecInfo = tlsSecInfo;

            fileServer = new FileServer(this, server.GetAvailableFiles(), DebugLog);

            if (server.fileTimeout != null)
                fileServer.Timeout = (long)server.fileTimeout;

            fileServer.SetFileReadyHandler(server.fileReadyHandler, server.fileReadyHandlerParameter);

            Thread workerThread = new Thread(HandleConnection);

            workerThread.Start();
        }

        /// <summary>
        /// Gets the connection parameters.
        /// </summary>
        /// <returns>The connection parameters used by the server.</returns>
        public ApplicationLayerParameters GetApplicationLayerParameters()
        {
            return alParameters;
        }

        /// <summary>
        /// Resets the T3 timeout value based on the current system time.
        /// </summary>
        /// <param name="currentTime">The current time in milliseconds, used to set the next T3 timeout.</param>
        private void ResetT3Timeout(UInt64 currentTime)
        {
            nextT3Timeout = (UInt64)SystemUtils.currentTimeMillis() + (UInt64)(apciParameters.T3 * 1000);
        }

        /// <summary>
        /// Checks whether the T3 timeout has occurred based on the current system time.
        /// </summary>
        /// <param name="currentTime">The current time in milliseconds, used to compare with the next T3 timeout.</param>
        /// <returns>
        /// <c>true</c> if the T3 timeout has occurred; otherwise, <c>false</c>.
        /// </returns>
        private bool CheckT3Timeout(UInt64 currentTime)
        {
            if (waitingForTestFRcon)
                return false;

            if (nextT3Timeout > (currentTime + (UInt64)(apciParameters.T3 * 1000)))
            {
                /* timeout value not plausible (maybe system time changed) */
                ResetT3Timeout(currentTime);
            }

            if (currentTime > nextT3Timeout)
                return true;
            else
                return false;
        }

        /// <summary>
        /// Resets the Test Frame Reception Confirmation (TestFRCon) timeout value based on the current system time.
        /// </summary>
        /// <param name="currentTime">The current time in milliseconds, used to set the next TestFRCon timeout.</param>
        private void ResetTestFRConTimeout(UInt64 currentTime)
        {
            nextTestFRConTimeout = currentTime + (UInt64)(apciParameters.T1 * 1000);
        }

        /// <summary>
        /// Checks whether the Test Frame Reception Confirmation (TestFRCon) timeout has occurred based on the current system time.
        /// </summary>
        /// <param name="currentTime">The current time in milliseconds, used to compare with the next TestFRCon timeout.</param>
        /// <returns>
        /// <c>true</c> if the TestFRCon timeout has occurred; otherwise, <c>false</c>.
        /// </returns>
        private bool CheckTestFRConTimeout(UInt64 currentTime)
        {
            if (nextTestFRConTimeout > (currentTime + (UInt64)(apciParameters.T1 * 1000)))
            {
                /* timeout value not plausible (maybe system time changed) */
                ResetTestFRConTimeout(currentTime);
            }

            if (currentTime > nextTestFRConTimeout)
                return true;
            else
                return false;
        }

        /// <summary>
        /// Flag indicating that this connection is the active connection.
        /// The active connection is the only connection that is answering
        /// application layer requests and sends cyclic, and spontaneous messages.
        /// </summary>
        private bool isActive = false;

        /// <summary>
        /// Gets or sets a value indicating whether this connection is active.
        /// The active connection is the only connection that is answering
        /// application layer requests and sends cyclic, and spontaneous messages.
        /// </summary>
        /// <value><c>true</c> if this instance is active; otherwise, <c>false</c>.</value>
        public bool IsActive
        {
            get
            {
                return isActive;
            }
            set
            {

                if (isActive != value)
                {

                    isActive = value;

                    if (isActive)
                    {
                        DebugLog("is active");
                        state = MasterConnectionState.M_CON_STATE_STARTED;
                    }
                    else
                        DebugLog("is not active");
                }
            }
        }

        public MasterConnectionState State { get => state; set => state = value; }

        private Socket socket;
        private Stream socketStream;

        private bool running = false;

        private bool debugOutput = true;

        private int readState = 0; /* 0 - idle, 1 - start received, 2 - reading remaining bytes */
        private int currentReadPos = 0;
        private int currentReadMsgLength = 0;
        private int remainingReadLength = 0;
        private long currentReadTimeout = 0;

        /// <summary>
        /// Receives a message from the socket stream, handling timeout and message parsing.
        /// </summary>
        private int receiveMessage(byte[] buffer)
        {
            /* check receive timeout */
            if (readState != 0)
            {
                if (SystemUtils.currentTimeMillis() > currentReadTimeout)
                {
                    DebugLog("Receive timeout!");
                    return -1;
                }
            }

            if (socket.Poll(50, SelectMode.SelectRead))
            {

                if (readState == 0)
                {
                    /* wait for start byte */
                    if (socketStream.Read(buffer, 0, 1) != 1)
                        return -1;

                    if (buffer[0] != 0x68)
                    {
                        DebugLog("Missing SOF indicator!");

                        return -1;
                    }

                    readState = 1;
                }

                if (readState == 1)
                {
                    /* read length byte */
                    if (socketStream.Read(buffer, 1, 1) != 1)
                        return 0;

                    currentReadMsgLength = buffer[1];
                    remainingReadLength = currentReadMsgLength;
                    currentReadPos = 2;

                    readState = 2;
                }

                if (readState == 2)
                {
                    int readLength = socketStream.Read(buffer, currentReadPos, remainingReadLength);

                    if (readLength == remainingReadLength)
                    {
                        readState = 0;
                        currentReadTimeout = 0;
                        return 2 + currentReadMsgLength;
                    }
                    else
                    {
                        currentReadPos += readLength;
                        remainingReadLength = remainingReadLength - readLength;
                    }
                }

                if (currentReadTimeout == 0)
                {
                    currentReadTimeout = SystemUtils.currentTimeMillis() + server.ReceiveTimeout;
                }
            }

            return 0;
        }

        /// <summary>
        /// Sends an S message via the socket stream.
        /// </summary>
        private void SendSMessage()
        {
            DebugLog("Send S message");

            byte[] msg = new byte[6];

            msg[0] = 0x68;
            msg[1] = 0x04;
            msg[2] = 0x01;
            msg[3] = 0;

            lock (socketStream)
            {
                msg[4] = (byte)((receiveCount % 128) * 2);
                msg[5] = (byte)(receiveCount / 128);

                try
                {
                    socketStream.Write(msg, 0, msg.Length);
                }
                catch (System.IO.IOException)
                {
                    running = false;
                }
            }
        }

        /// <summary>
        /// Sends an I message containing the ASDU via the socket stream.
        /// </summary>
        /// <param name="asdu">The ASDU object containing the message data to be sent.</param>
        /// <returns>
        /// The updated send count, which is incremented after sending the message.
        /// </returns>
        private int SendIMessage(BufferFrame asdu)
        {

            byte[] buffer = asdu.GetBuffer();

            int msgSize = asdu.GetMsgSize(); /* ASDU size + ACPI size */

            buffer[0] = 0x68;

            /* set size field */
            buffer[1] = (byte)(msgSize - 2);

            buffer[2] = (byte)((sendCount % 128) * 2);
            buffer[3] = (byte)(sendCount / 128);

            buffer[4] = (byte)((receiveCount % 128) * 2);
            buffer[5] = (byte)(receiveCount / 128);

            try
            {
                lock (socketStream)
                {
                    socketStream.Write(buffer, 0, msgSize);
                    DebugLog("SEND I (size = " + msgSize + ") : " + BitConverter.ToString(buffer, 0, msgSize));
                    sendCount = (sendCount + 1) % 32768;
                    unconfirmedReceivedIMessages = 0;
                    timeoutT2Triggered = false;
                }
            }
            catch (System.IO.IOException)
            {
                running = false;
            }

            return sendCount;
        }

        /// <summary>
        /// Checks if there are any unconfirmed messages in the low-priority or high-priority queues.
        /// </summary>
        /// <returns>
        /// <c>true</c> if there are unconfirmed messages in either the low-priority or high-priority queues; 
        /// <c>false</c> if no unconfirmed messages are found.
        /// </returns>
        public bool MasterConnection_hasUnconfirmedMessages()
        {
            bool retVal = false;

            if (lowPrioQueue != null)
            {
                if (lowPrioQueue.MessageQueue_hasUnconfirmedIMessages())
                    return true;

                if (highPrioQueue.MessageQueue_hasUnconfirmedIMessages())
                    return true;
            }

            return retVal;
        }

        /// <summary>
        /// Checks whether the sent ASDU buffer is full.
        /// </summary>
        /// <returns>
        /// <c>true</c> if the sent ASDU buffer is full; 
        /// <c>false</c> if the buffer is not full.
        /// </returns>
        private bool isSentBufferFull()
        {

            if (oldestSentASDU == -1)
                return false;

            int newIndex = (newestSentASDU + 1) % maxSentASDUs;

            if (newIndex == oldestSentASDU)
                return true;
            else
                return false;
        }

        /// <summary>
        /// Prints the contents of the sent ASDU buffer for debugging purposes.
        /// </summary>
        private void PrintSendBuffer()
        {
            if (debugOutput)
            {
                if (oldestSentASDU != -1)
                {

                    int currentIndex = oldestSentASDU;

                    int nextIndex = 0;

                    DebugLog("------k-buffer------");

                    do
                    {
                        DebugLog(currentIndex + " : S " + sentASDUs[currentIndex].seqNo + " : time " +
                            sentASDUs[currentIndex].sentTime + " : " + sentASDUs[currentIndex].queueIndex);

                        if (currentIndex == newestSentASDU)
                            nextIndex = -1;
                        else
                            currentIndex = (currentIndex + 1) % maxSentASDUs;

                    } while (nextIndex != -1);

                    DebugLog("--------------------");

                }
            }
        }

        /// <summary>
        /// Sends the next available ASDU from the low-priority queue, if the sent ASDU buffer is not full.
        /// </summary>
        private void sendNextAvailableASDU()
        {
            lock (sentASDUs)
            {
                if (isSentBufferFull())
                    return;

                long timestamp;
                int index;

                lowPrioQueue.LockASDUQueue();
                BufferFrame asdu = lowPrioQueue.GetNextWaitingASDU(out timestamp, out index);

                try
                {
                    if (asdu != null)
                    {
                        int currentIndex = 0;

                        if (oldestSentASDU == -1)
                        {
                            oldestSentASDU = 0;
                            newestSentASDU = 0;
                        }
                        else
                        {
                            currentIndex = (newestSentASDU + 1) % maxSentASDUs;
                        }

                        sentASDUs[currentIndex].entryTime = timestamp;
                        sentASDUs[currentIndex].queueIndex = index;
                        sentASDUs[currentIndex].seqNo = SendIMessage(asdu);
                        sentASDUs[currentIndex].sentTime = SystemUtils.currentTimeMillis();

                        newestSentASDU = currentIndex;

                        PrintSendBuffer();
                    }
                }
                finally
                {
                    lowPrioQueue.UnlockASDUQueue();
                }
            }
        }

        /// <summary>
        /// Sends the next available high-priority ASDU from the high-priority queue, if the sent ASDU buffer is not full.
        /// </summary>
        /// <returns>
        /// <c>true</c> if a high-priority ASDU was successfully sent; 
        /// <c>false</c> if the sent ASDU buffer is full and no message was sent.
        /// </returns>
        private bool sendNextHighPriorityASDU()
        {
            lock (sentASDUs)
            {
                if (isSentBufferFull())
                    return false;

                long timestamp;
                int index;

                highPrioQueue.LockASDUQueue();

                BufferFrame asdu = highPrioQueue.GetNextHighPriorityWaitingASDU(out timestamp, out index);

                try
                {
                    if (asdu != null)
                    {
                        int currentIndex = 0;

                        if (oldestSentASDU == -1)
                        {
                            oldestSentASDU = 0;
                            newestSentASDU = 0;

                        }
                        else
                        {
                            currentIndex = (newestSentASDU + 1) % maxSentASDUs;
                        }

                        sentASDUs[currentIndex].entryTime = timestamp;
                        sentASDUs[currentIndex].queueIndex = index;
                        sentASDUs[currentIndex].seqNo = SendIMessage(asdu);
                        sentASDUs[currentIndex].sentTime = SystemUtils.currentTimeMillis();

                        newestSentASDU = currentIndex;

                        PrintSendBuffer();
                    }
                }
                finally
                {
                    highPrioQueue.UnlockASDUQueue();
                }
              
            }

            return true;
        }

        /// <summary>
        ///  Send all high-priority ASDUs and the last waiting ASDU from the low-priority queue.
        ///  Returns true if ASDUs are still waiting.This can happen when there are more ASDUs
        ///  in the event (low-priority) buffer, or the connection is unavailable to send the high-priority
        ///  ASDUs (congestion or connection lost).
        /// </summary>
        private bool SendWaitingASDUs()
        {

            lock (highPrioQueue)
            {
                /* send all available high priority ASDUs first */
                while (highPrioQueue.IsHighPriorityAsduAvailable())
                {

                    if (sendNextHighPriorityASDU() == false)
                        return true;

                    if (running == false)
                        return true;
                }
            }

            /* send messages from low-priority queue */
            sendNextAvailableASDU();

            if (lowPrioQueue.NumberOfAsduInQueue > 0)
                return true;
            else
                return false;
        }

        /// <summary>
        /// Encodes and enqueues an ASDU for transmission, then attempts to send any waiting ASDUs.
        /// </summary>
        /// <param name="asdu">
        /// The ASDU to be encoded and enqueued for sending.
        /// </param>
        private void SendASDUInternal(ASDU asdu)
        {
            if (isActive)
            {
                lock (highPrioQueue)
                {

                    BufferFrame frame = new BufferFrame(new byte[256], 6);

                    asdu.Encode(frame, alParameters);

                    highPrioQueue.EnqueueAsdu(asdu);
                }

                SendWaitingASDUs();
            }
        }

        /// <summary>
        /// Send a response ASDU over this connection
        /// </summary>
        /// <exception cref="ConnectionException">Throws an exception if the connection is no longer active (e.g. because it has been closed by the other side).</exception>
        /// <param name="asdu">The ASDU to send</param>
        public void SendASDU(ASDU asdu)
        {
            if (isActive)
                SendASDUInternal(asdu);
            else
                throw new ConnectionException("Connection not active");
        }

        /// <summary>
        /// Sends an Activation Confirmation (ACT_CON) ASDU with the specified negative flag.
        /// </summary>
        /// <param name="asdu">
        /// The ASDU to be sent as an Activation Confirmation (ACT_CON).
        /// </param>
        /// <param name="negative">
        /// A boolean flag indicating whether the Activation Confirmation is negative.
        /// </param>
        public void SendACT_CON(ASDU asdu, bool negative)
        {
            asdu.Cot = CauseOfTransmission.ACTIVATION_CON;
            asdu.IsNegative = negative;

            SendASDU(asdu);
        }

        /// <summary>
        /// Sends an Activation Termination (ACT_TERM) ASDU with a negative flag set to false.
        /// </summary>
        /// <param name="asdu">
        /// The ASDU to be sent as an Activation Termination (ACT_TERM).
        /// </param>
        public void SendACT_TERM(ASDU asdu)
        {
            asdu.Cot = CauseOfTransmission.ACTIVATION_TERMINATION;
            asdu.IsNegative = false;

            SendASDU(asdu);
        }

        // <summary>
        /// Handles the received ASDU (Application Service Data Unit) based on its TypeID and Cause of Transmission (COT).
        /// It processes various commands such as interrogation, counter interrogation, read, clock synchronization, test, and reset commands.
        /// </summary>
        /// <param name="asdu">
        /// The ASDU received that needs to be processed.
        /// </param>
        private void HandleASDU(ASDU asdu)
        {
            DebugLog("Handle received ASDU");

            bool messageHandled = false;

            switch (asdu.TypeId)
            {

                case TypeID.C_IC_NA_1: /* 100 - interrogation command */

                    DebugLog("Rcvd interrogation command C_IC_NA_1\n");

                    if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.DEACTIVATION))
                    {
                        if (server.interrogationHandler != null)
                        {

                            InterrogationCommand irc = (InterrogationCommand)asdu.GetElement(0);

                            if (irc != null)
                            {
                                /* Verify IOA = 0 */
                                if (irc.ObjectAddress != 0)
                                {
                                    DebugLog("CS104 SLAVE: interrogation command has invalid IOA - should be 0\n");
                                    asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                    messageHandled = true;
                                }
                                else
                                {
                                    if (server.interrogationHandler(server.InterrogationHandlerParameter, this, asdu, irc.QOI))
                                        messageHandled = true;
                                }
                            }
                        }
                    }
                    else
                    {
                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                        asdu.IsNegative = true;
                        SendASDUInternal(asdu);
                        messageHandled = true;
                    }

                    break;

                case TypeID.C_CI_NA_1: /* 101 - counter interrogation command */

                    DebugLog("Rcvd counter interrogation command C_CI_NA_1\n");

                    if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.DEACTIVATION))
                    {
                        if (server.counterInterrogationHandler != null)
                        {

                            CounterInterrogationCommand cic = (CounterInterrogationCommand)asdu.GetElement(0);

                            if (cic != null)
                            {
                                /* Verify IOA = 0 */
                                if (cic.ObjectAddress != 0)
                                {
                                    DebugLog("CS104 SLAVE: counter interrogation command has invalid IOA - should be 0\n");
                                    asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                    messageHandled = true;
                                }
                                else
                                {
                                    if (server.interrogationHandler(server.InterrogationHandlerParameter, this, asdu, cic.QCC))
                                        messageHandled = true;
                                }
                            }
                        }
                    }
                    else
                    {
                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                        asdu.IsNegative = true;
                        SendASDUInternal(asdu);
                        messageHandled = true;
                    }

                    break;

                case TypeID.C_RD_NA_1: /* 102 - read command */

                    DebugLog("Rcvd read command C_RD_NA_1\n");

                    if (asdu.Cot == CauseOfTransmission.REQUEST)
                    {

                        DebugLog("Read request for object: " + asdu.Ca);

                        if (server.readHandler != null)
                        {
                            ReadCommand rc = (ReadCommand)asdu.GetElement(0);

                            if (rc != null)
                            {
                                if (server.readHandler(server.readHandlerParameter, this, asdu, rc.ObjectAddress))
                                    messageHandled = true;
                            }
                        }
                    }
                    else
                    {
                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                        asdu.IsNegative = true;
                        SendASDUInternal(asdu);
                        messageHandled = true;
                    }

                    break;

                case TypeID.C_CS_NA_1: /* 103 - Clock synchronization command */

                    DebugLog("Rcvd clock sync command C_CS_NA_1\n");

                    if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                    {

                        if (server.clockSynchronizationHandler != null)
                        {

                            ClockSynchronizationCommand csc = (ClockSynchronizationCommand)asdu.GetElement(0);

                            if (csc != null)
                            {
                                /* Verify IOA = 0 */
                                if (csc.ObjectAddress != 0)
                                {
                                    DebugLog("CS104 SLAVE: Clock synchronization command has invalid IOA - should be 0\n");
                                    asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                }
                                else
                                {
                                    if (server.clockSynchronizationHandler(server.clockSynchronizationHandlerParameter,
                                        this, asdu, csc.NewTime))
                                    {
                                        csc.ObjectAddress = 0;
                                        asdu.AddInformationObject(csc);
                                        asdu.Cot = CauseOfTransmission.ACTIVATION_CON;
                                        SendASDUInternal(asdu);
                                    }
                                    else
                                    {
                                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                                        asdu.IsNegative = true;
                                        SendASDUInternal(asdu);
                                    }

                                }

                                messageHandled = true;
                            }
                        }

                    }
                    else
                    {
                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                        asdu.IsNegative = true;
                        SendASDUInternal(asdu);
                        messageHandled = true;
                    }

                    break;

                case TypeID.C_TS_NA_1: /* 104 - test command */

                    DebugLog("Rcvd test command C_TS_NA_1\n");

                    if (allowTestCommand)
                    {
                        if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                        {
                            TestCommand tc = (TestCommand)asdu.GetElement(0);

                            /* Verify IOA = 0 */
                            if (tc.ObjectAddress != 0)
                            {
                                DebugLog("CS104 SLAVE: test command has invalid IOA - should be 0\n");
                                asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                            }
                            else
                            {
                                asdu.Cot = CauseOfTransmission.ACTIVATION_CON;
                                SendASDUInternal(asdu);
                            }

                            messageHandled = true;
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            messageHandled = true;
                            SendASDUInternal(asdu);
                        }
                    }
                    else
                    {
                        /* this command is not supported/allowed for IEC 104 */
                        DebugLog("CS104 SLAVE: Rcvd test command C_TS_NA_1 -> not allowed\n");
                        messageHandled = false;
                    }

                    break;

                case TypeID.C_RP_NA_1: /* 105 - Reset process command */

                    DebugLog("Rcvd reset process command C_RP_NA_1\n");

                    if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                    {

                        if (server.resetProcessHandler != null)
                        {

                            ResetProcessCommand rpc = (ResetProcessCommand)asdu.GetElement(0);

                            if (rpc != null)
                            {
                                /* Verify IOA = 0 */
                                if (rpc.ObjectAddress != 0)
                                {
                                    DebugLog("CS104 SLAVE: reset process command has invalid IOA - should be 0\n");
                                    asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                    messageHandled = true;
                                }
                                else
                                {
                                    if (server.interrogationHandler(server.InterrogationHandlerParameter, this, asdu, rpc.QRP))
                                        messageHandled = true;
                                }
                            }
                        }

                    }
                    else
                    {
                        asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                        asdu.IsNegative = true;
                        SendASDUInternal(asdu);
                        messageHandled = true;
                    }

                    break;

                case TypeID.C_CD_NA_1: /* 106 - Delay acquisition command */

                    DebugLog("Rcvd delay acquisition command C_CD_NA_1\n");

                    if (allowDelayAcquisition)
                    {
                        if ((asdu.Cot == CauseOfTransmission.ACTIVATION) || (asdu.Cot == CauseOfTransmission.SPONTANEOUS))
                        {
                            if (server.delayAcquisitionHandler != null)
                            {

                                DelayAcquisitionCommand dac = (DelayAcquisitionCommand)asdu.GetElement(0);

                                if (dac != null)
                                {
                                    /* Verify IOA = 0 */
                                    if (dac.ObjectAddress != 0)
                                    {
                                        DebugLog("CS104 SLAVE: delay acquisition command has invalid IOA - should be 0\n");
                                        asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                        messageHandled = true;
                                    }
                                    else
                                    {
                                        if (server.delayAcquisitionHandler(server.delayAcquisitionHandlerParameter,
                                                this, asdu, dac.Delay))
                                            messageHandled = true;
                                    }
                                }
                            }
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                            messageHandled = true;
                            SendASDUInternal(asdu);
                        }
                    }
                    else
                    {
                        /* this command is not supported/allowed for IEC 104 */
                        DebugLog("CS104 SLAVE: Rcvd delay acquisition command C_CD_NA_1 -> not allowed\n");
                        messageHandled = false;
                    }

                    break;

                case TypeID.C_TS_TA_1: /* 107 - test command with timestamp */

                    DebugLog("Rcvd test command with CP56Time2a C_TS_TA_1\n");

                    if (asdu.Cot == CauseOfTransmission.ACTIVATION)
                    {
                        TestCommandWithCP56Time2a tc = (TestCommandWithCP56Time2a)asdu.GetElement(0);

                        if (tc.ObjectAddress != 0)
                        {
                            DebugLog("CS104 SLAVE: test command with CP56Time2a has invalid IOA - should be 0\n");
                            asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                        }
                        else
                        {
                            asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                            asdu.IsNegative = true;
                        }

                        messageHandled = true;
                    }
                    else
                        asdu.Cot = CauseOfTransmission.ACTIVATION_CON;

                    messageHandled = true;
                    SendASDUInternal(asdu);

                    break;

                default: /* no special handler available -> use default handler */
                    break;
            }

            if (messageHandled == false)
                messageHandled = fileServer.HandleFileAsdu(asdu);

            if ((messageHandled == false) && (server.asduHandler != null))
                if (server.asduHandler(server.asduHandlerParameter, this, asdu))
                    messageHandled = true;

            if (messageHandled == false)
            {
                asdu.Cot = CauseOfTransmission.UNKNOWN_TYPE_ID;
                asdu.IsNegative = true;
                SendASDUInternal(asdu);
            }

        }

        /// <summary>
        /// Checks if the given sequence number is valid based on the sequence numbers of previously sent ASDUs.
        /// This method ensures that the sequence numbers follow the correct order and handles the possibility of a sequence number overflow.
        /// </summary>
        /// <param name="seqNo">
        /// The sequence number to be checked for validity.
        /// </param>
        /// <returns>
        /// Returns <c>true</c> if the sequence number is valid; otherwise, <c>false</c> if it is out of range or invalid.
        /// </returns>
        private bool CheckSequenceNumber(int seqNo)
        {
            lock (sentASDUs)
            {
                /* check if received sequence number is valid */

                bool seqNoIsValid = false;
                bool counterOverflowDetected = false;
                int oldestValidSeqNo = -1;

                if (oldestSentASDU == -1)
                { /* if k-Buffer is empty */
                    if (seqNo == sendCount)
                        seqNoIsValid = true;
                }
                else
                {
                    /* Two cases are required to reflect sequence number overflow */
                    if (sentASDUs[oldestSentASDU].seqNo <= sentASDUs[newestSentASDU].seqNo)
                    {
                        if ((seqNo >= sentASDUs[oldestSentASDU].seqNo) &&
                        (seqNo <= sentASDUs[newestSentASDU].seqNo))
                        {
                            seqNoIsValid = true;
                        }
                    }
                    else
                    {
                        if ((seqNo >= sentASDUs[oldestSentASDU].seqNo) ||
                        (seqNo <= sentASDUs[newestSentASDU].seqNo))
                        {
                            seqNoIsValid = true;
                        }

                        counterOverflowDetected = true;
                    }

                    if (sentASDUs[oldestSentASDU].seqNo == 0)
                        oldestValidSeqNo = 32767;
                    else
                        oldestValidSeqNo = sentASDUs[oldestSentASDU].seqNo - 1;

                    if (oldestValidSeqNo == seqNo)
                        seqNoIsValid = true;
                }

                if (seqNoIsValid == false)
                {
                    DebugLog("Received sequence number out of range");
                    return false;
                }

                if (oldestSentASDU != -1)
                {
                    /* remove confirmed messages from list */
                    do
                    {
                        /* skip removing messages if confirmed message was already removed */
                        if (counterOverflowDetected == false)
                        {
                            if (seqNo < sentASDUs[oldestSentASDU].seqNo)
                                break;
                        }

                        if (seqNo == oldestValidSeqNo)
                            break;

                        /* remove from server (low-priority) queue if required */
                        if (sentASDUs[oldestSentASDU].queueIndex != -1)
                        {
                            lowPrioQueue.MarkASDUAsConfirmed(sentASDUs[oldestSentASDU].queueIndex,
                                sentASDUs[oldestSentASDU].entryTime);
                        }

                        if (sentASDUs[oldestSentASDU].seqNo == seqNo)
                        {
                            /* we arrived at the seq# that has been confirmed */

                            if (oldestSentASDU == newestSentASDU)
                                oldestSentASDU = -1;
                            else
                                oldestSentASDU = (oldestSentASDU + 1) % maxSentASDUs;

                            break;
                        }

                        oldestSentASDU = (oldestSentASDU + 1) % maxSentASDUs;

                        int checkIndex = (newestSentASDU + 1) % maxSentASDUs;

                        if (oldestSentASDU == checkIndex)
                        {
                            oldestSentASDU = -1;
                            break;
                        }

                    } while (true);
                }
            }

            return true;
        }

        /// <summary>
        /// Processes an incoming message based on its type and takes appropriate actions according to the message content.
        /// This method handles different types of messages, such as I-message, TESTFR_ACT, STARTDT_ACT, STOPDT_ACT, 
        /// and S-message, as well as error handling and state transitions of the connection.
        /// </summary>
        /// <param name="buffer">
        /// The byte array containing the incoming message to be processed.
        /// </param>
        /// <param name="msgSize">
        /// The size of the incoming message (in bytes).
        /// </param>
        /// <returns>
        /// Returns <c>true</c> if the message was processed successfully; otherwise, <c>false</c> if the message was invalid
        /// or if an error occurred while processing it.
        /// </returns>
        private bool HandleMessage(byte[] buffer, int msgSize)
        {
            UInt64 currentTime = (UInt64)SystemUtils.currentTimeMillis();

            if (msgSize >= 3)
            {
                if (buffer[0] != 0x68)
                {
                    DebugLog("Invalid START character!");
                    return false;
                }

                byte lengthOfApdu = buffer[1];

                if (lengthOfApdu != msgSize - 2)
                {
                    DebugLog("Invalid length of APDU");
                    return false;
                }

                if ((buffer[2] & 1) == 0) /* I message */
                {
                    if (msgSize < 7)
                    {
                        DebugLog("I msg too small!");
                        return false;
                    }

                    if (state != MasterConnectionState.M_CON_STATE_STARTED)
                    {
                        DebugLog("Received I message while connection not active -> close connection");
                        return false;
                    }

                    if (timeoutT2Triggered == false)
                    {
                        timeoutT2Triggered = true;
                        lastConfirmationTime = currentTime; /* start timeout T2 */
                    }

                    int frameSendSequenceNumber = ((buffer[3] * 0x100) + (buffer[2] & 0xfe)) / 2;
                    int frameRecvSequenceNumber = ((buffer[5] * 0x100) + (buffer[4] & 0xfe)) / 2;

                    DebugLog("Received I frame: N(S) = " + frameSendSequenceNumber + " N(R) = " + frameRecvSequenceNumber);

                    /* check the receive sequence number N(R) - connection will be closed on an unexpected value */
                    if (frameSendSequenceNumber != receiveCount)
                    {
                        DebugLog("Sequence error: Close connection!");
                        return false;
                    }

                    if (CheckSequenceNumber(frameRecvSequenceNumber) == false)
                    {
                        DebugLog("Sequence number check failed");
                        return false;
                    }

                    receiveCount = (receiveCount + 1) % 32768;
                    unconfirmedReceivedIMessages++;

                    if (isActive)
                    {
                        try
                        {
                            ASDU asdu = new ASDU(alParameters, buffer, 6, msgSize);

                            /* push to handler thread for processing */
                            DebugLog("Enqueue received I-message for processing");
                            receivedASDUs.Enqueue(asdu);
                        }
                        catch (ASDUParsingException e)
                        {
                            DebugLog("ASDU parsing failed: " + e.Message);
                            return false;
                        }
                    }
                    else
                    {
                        /* connection not active */
                        DebugLog("Connection not active -> close connection");

                        return false;
                    }
                }

                /* Check for TESTFR_ACT message */
                else if ((buffer[2] & 0x43) == 0x43)
                {
                    DebugLog("Send TESTFR_CON");

                    socketStream.Write(TESTFR_CON_MSG, 0, TESTFR_CON_MSG.Length);
                }

                /* Check for STARTDT_ACT message */
                else if ((buffer[2] & 0x07) == 0x07)
                {
                    if (isActive == false)
                    {
                        isActive = true;

                        server.Activated(this);
                    }

                    DebugLog("Send STARTDT_CON");

                    socketStream.Write(STARTDT_CON_MSG, 0, TESTFR_CON_MSG.Length);
                }

                /* Check for STOPDT_ACT message */
                else if ((buffer[2] & 0x13) == 0x13)
                {
                    DebugLog("Received STARTDT_ACT");

                    if (isActive == true)
                    {
                        isActive = false;

                        server.Deactivated(this);
                    }

                    /* Send S-Message to confirm all outstanding messages */

                    if (unconfirmedReceivedIMessages > 0)
                    {
                        lastConfirmationTime = currentTime;
                        unconfirmedReceivedIMessages = 0;
                        timeoutT2Triggered = false;
                        SendSMessage();
                    }

                    if (MasterConnection_hasUnconfirmedMessages())
                    {
                        DebugLog("CS104 SLAVE: Unconfirmed messages after STOPDT_ACT -> pending unconfirmed stopped state\n");
                    }
                    else
                    {
                        DebugLog("Send STOPDT_CON");

                        state = MasterConnectionState.M_CON_STATE_STOPPED;

                        try
                        {
                            socketStream.Write(STOPDT_CON_MSG, 0, STOPDT_CON_MSG.Length);
                        }
                        catch (IOException)
                        {
                            DebugLog("Failed to send STOPDT_CON");
                            return false;
                        }
                    }
                }

                /* Check for TESTFR_CON message */
                else if ((buffer[2] & 0x83) == 0x83)
                {
                    DebugLog("Recv TESTFR_CON");

                    waitingForTestFRcon = false;

                    ResetT3Timeout(currentTime);
                }

                /* S-message */
                else if (buffer[2] == 0x01)
                {
                    int seqNo = (buffer[4] + buffer[5] * 0x100) / 2;

                    DebugLog("Recv S(" + seqNo + ") (own sendcounter = " + sendCount + ")");

                    if (CheckSequenceNumber(seqNo) == false)
                    {
                        DebugLog("S message - sequence number mismatch");
                        return false;
                    }

                    if (state == MasterConnectionState.M_CON_STATE_UNCONFIRMED_STOPPED)
                    {
                        if (MasterConnection_hasUnconfirmedMessages() == false)
                        {
                            state = MasterConnectionState.M_CON_STATE_STOPPED;

                            DebugLog("Send STOPDT_CON\n");

                            try
                            {
                                socketStream.Write(STOPDT_CON_MSG, 0, STOPDT_CON_MSG.Length);
                            }
                            catch (IOException ex)
                            {
                                DebugLog("Failed to send STOPDT_CON: " + ex.Message);
                                return false;
                            }
                        }
                    }
                    else if (state == MasterConnectionState.M_CON_STATE_STOPPED)
                    {
                        DebugLog("S message sin stopped state -> active close\n");
                        /* actively close connection */
                        return false;
                    }
                }
                else
                {
                    DebugLog("Unknown message - IGNORE");
                    return true;
                }

                ResetT3Timeout(currentTime);

                return true;
            }
            else
            {
                DebugLog("Invalid message (too small)");
                return false;
            }
        }

        /// <summary>
        /// Handles various timeout conditions in the communication process, such as T3 timeouts, TESTFR_CON timeouts, 
        /// and I-message timeouts. It checks for timeouts, sends necessary messages (such as TESTFR_ACT or S-message), 
        /// and ensures the connection is managed correctly based on timeout events.
        /// </summary>
        /// <returns>
        /// Returns <c>true</c> if no timeouts were detected or handled successfully, otherwise returns <c>false</c> 
        /// if a timeout condition was detected and the connection should be closed or processed further.
        /// </returns>
        private bool handleTimeouts()
        {
            UInt64 currentTime = (UInt64)SystemUtils.currentTimeMillis();

            if (CheckT3Timeout(currentTime))
            {
                try
                {
                    socketStream.Write(TESTFR_ACT_MSG, 0, TESTFR_ACT_MSG.Length);

                    DebugLog("U message T3 timeout");
                    ResetT3Timeout(currentTime);
                }
                catch (System.IO.IOException)
                {
                    running = false;
                }

                waitingForTestFRcon = true;

                ResetTestFRConTimeout(currentTime);
            }

            /* Check for TEST FR con timeout */
            if (waitingForTestFRcon)
            {
                if (CheckTestFRConTimeout(currentTime))
                {
                    DebugLog("Timeout for TESTFR_CON message");

                    return false;
                }
            }

            if (unconfirmedReceivedIMessages > 0)
            {
                if ((currentTime - lastConfirmationTime) >= (UInt64)(apciParameters.T2 * 1000))
                {

                    lastConfirmationTime = currentTime;
                    unconfirmedReceivedIMessages = 0;
                    timeoutT2Triggered = false;
                    SendSMessage();
                }
            }

            /* check if counterpart confirmed I messages */
            lock (sentASDUs)
            {
                if (oldestSentASDU != -1)
                {
                    if (((long)currentTime - sentASDUs[oldestSentASDU].sentTime) >= (apciParameters.T1 * 1000))
                    {
                        PrintSendBuffer();
                        DebugLog("I message timeout for " + oldestSentASDU + " seqNo: " + sentASDUs[oldestSentASDU].seqNo);
                        return false;
                    }
                }
            }

            return true;
        }

        /// <summary>
        /// Compares two byte arrays for equality by checking if they are of the same length and if each byte in the arrays is equal.
        /// </summary>
        /// <param name="array1">
        /// The first byte array to be compared.
        /// </param>
        /// <param name="array2">
        /// The second byte array to be compared.
        /// </param>
        /// <returns>
        /// Returns <c>true</c> if the byte arrays are equal (same length and same content), otherwise returns <c>false</c>.
        /// </returns>
        private bool AreByteArraysEqual(byte[] array1, byte[] array2)
        {
            if (array1.Length == array2.Length)
            {
                for (int i = 0; i < array1.Length; i++)
                {
                    if (array1[i] != array2[i])
                        return false;
                }

                return true;
            }
            else
                return false;
        }

        /// <summary>
        /// Callback method to handle certificate validation in SSL/TLS communication. It checks various conditions 
        /// such as certificate chain validation, specific certificate validation, and SSL policy errors to determine
        /// whether the certificate is valid for the current connection.
        /// </summary>
        /// <param name="sender">
        /// The source of the event. This is typically the SSL/TLS connection.
        /// </param>
        /// <param name="cert">
        /// The certificate being validated.
        /// </param>
        /// <param name="chain">
        /// The certificate chain associated with the certificate.
        /// </param>
        /// <param name="sslPolicyErrors">
        /// The SSL policy errors that occurred during certificate validation.
        /// </param>
        /// <returns>
        /// Returns <c>true</c> if the certificate is considered valid based on the specified criteria; otherwise, returns <c>false</c>.
        /// </returns>
        public bool CertificateValidationCallback(object sender, X509Certificate cert, X509Chain chain, SslPolicyErrors sslPolicyErrors)
        {
            if (sslPolicyErrors == SslPolicyErrors.None || sslPolicyErrors == SslPolicyErrors.RemoteCertificateChainErrors)
            {
                if (tlsSecInfo.ChainValidation)
                {
                    X509Chain newChain = new X509Chain();

                    newChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                    newChain.ChainPolicy.RevocationFlag = X509RevocationFlag.ExcludeRoot;
                    newChain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllowUnknownCertificateAuthority;
                    newChain.ChainPolicy.VerificationTime = DateTime.Now;
                    newChain.ChainPolicy.UrlRetrievalTimeout = new TimeSpan(0, 0, 0);

                    foreach (X509Certificate2 caCert in tlsSecInfo.CaCertificates)
                        newChain.ChainPolicy.ExtraStore.Add(caCert);

                    bool certificateStatus = newChain.Build(new X509Certificate2(cert.GetRawCertData()));

                    if (certificateStatus == false)
                        return false;
                }

                if (tlsSecInfo.AllowOnlySpecificCertificates)
                {
                    foreach (X509Certificate2 allowedCert in tlsSecInfo.AllowedCertificates)
                    {
                        if (AreByteArraysEqual(allowedCert.GetCertHash(), cert.GetCertHash()))
                        {
                            return true;
                        }
                    }

                    return false;
                }

                return true;
            }
            else
                return false;
        }

        /// <summary>
        /// Handles the connection lifecycle, including setting up the SSL/TLS connection, 
        /// receiving and processing messages, managing timeouts, and closing the connection.
        /// </summary>
        private void HandleConnection()
        {
            byte[] bytes = new byte[300];

            try
            {
                try
                {
                    running = true;

                    if (tlsSecInfo != null)
                    {
                        DebugLog("Setup TLS");

                        RemoteCertificateValidationCallback validationCallback = CertificateValidationCallback;

                        if (tlsSecInfo.CertificateValidationCallback != null)
                            validationCallback = tlsSecInfo.CertificateValidationCallback;

                        SslStream sslStream = new SslStream(socketStream, true, validationCallback);

                        bool authenticationSuccess = false;

                        try
                        {
                            System.Security.Authentication.SslProtocols tlsVersion = System.Security.Authentication.SslProtocols.None;

                            if (tlsSecInfo != null)
                                tlsVersion = tlsSecInfo.TlsVersion;

                            DebugLog("Using TLS version: " + tlsVersion.ToString());

                            sslStream.AuthenticateAsServer(tlsSecInfo.OwnCertificate, true, tlsVersion, false);

                            if (sslStream.IsAuthenticated == true)
                            {
                                socketStream = sslStream;
                                authenticationSuccess = true;
                            }

                        }
                        catch (IOException e)
                        {
                            if (e.GetBaseException() != null)
                            {
                                DebugLog("TLS authentication error: " + e.GetBaseException().Message);
                            }
                            else
                            {
                                DebugLog("TLS authentication error: " + e.Message);
                            }
                        }

                        if (authenticationSuccess == true)
                            socketStream = sslStream;
                        else
                        {
                            DebugLog("TLS authentication failed");
                            running = false;
                        }
                    }

                    if (running)
                    {
                        socketStream.ReadTimeout = 50;

                        callbackThread = new Thread(ProcessASDUs);
                        callbackThread.Start();

                        ResetT3Timeout((UInt64)SystemUtils.currentTimeMillis());
                    }

                    while (running)
                    {
                        try
                        {
                            /* Receive the response from the remote device */
                            int bytesRec = receiveMessage(bytes);

                            if (bytesRec > 0)
                            {

                                DebugLog("RCVD: " + BitConverter.ToString(bytes, 0, bytesRec));

                                if (HandleMessage(bytes, bytesRec) == false)
                                {
                                    /* close connection on error */
                                    running = false;
                                }

                                if (unconfirmedReceivedIMessages >= apciParameters.W)
                                {
                                    lastConfirmationTime = (UInt64)SystemUtils.currentTimeMillis();
                                    unconfirmedReceivedIMessages = 0;
                                    timeoutT2Triggered = false;
                                    SendSMessage();
                                }
                            }
                            else if (bytesRec == -1)
                            {
                                running = false;
                            }
                        }
                        catch (System.IO.IOException)
                        {
                            running = false;
                        }

                        if (fileServer != null)
                            fileServer.HandleFileTransmission();

                        if (handleTimeouts() == false)
                            running = false;

                        if (running)
                        {
                            if (isActive)
                                SendWaitingASDUs();

                            Thread.Sleep(1);
                        }
                    }

                    isActive = false;

                    DebugLog("CLOSE CONNECTION!");

                    socket.Shutdown(SocketShutdown.Both);
                    socket.Close();

                    socketStream.Dispose();
                    socket.Dispose();

                    DebugLog("CONNECTION CLOSED!");

                }
                catch (ArgumentNullException ane)
                {
                    DebugLog("ArgumentNullException : " + ane.ToString());
                }
                catch (SocketException se)
                {
                    DebugLog("SocketException : " + se.ToString());
                }
                catch (Exception e)
                {
                    DebugLog("Unexpected exception : " + e.ToString());
                }

            }
            catch (Exception e)
            {
                DebugLog(e.ToString());
            }

            /* unmark unconfirmed messages in queue if k-buffer not empty */
            if (oldestSentASDU != -1)
                lowPrioQueue.UnmarkAllASDUs();

            server.Remove(this);

            if (callbackThreadRunning)
            {
                callbackThreadRunning = false;
                callbackThread.Join();
            }

            DebugLog("Connection thread finished");
        }

        void HandleRemoteCertificateValidationCallback(object sender, X509Certificate certificate, X509Chain chain, SslPolicyErrors sslPolicyErrors)
        {
        }

        public void Close()
        {
            running = false;
            state = MasterConnectionState.M_CON_STATE_STOPPED;
        }

        public void ASDUReadyToSend()
        {
            if (isActive)
                SendWaitingASDUs();
        }

    }

}
