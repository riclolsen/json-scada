/*
 *  Connection.cs
 *
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
using System.Collections.Generic;
using System.IO;
using System.Net;
using System.Net.Security;
using System.Net.Sockets;
using System.Security.Cryptography.X509Certificates;
using System.Threading;

namespace lib60870.CS104
{
    /// <summary>
    /// Connection event for CS 104 client (\ref Connection)
    /// </summary>
    public enum ConnectionEvent
    {
        /// <summary>
        /// The connection has been opened
        /// </summary>
        OPENED = 0,

        /// <summary>
        /// The connection has been closed
        /// </summary>
        CLOSED = 1,

        /// <summary>
        /// Conformation of START DT command received (server will send and accept application layer messages)
        /// </summary>
        STARTDT_CON_RECEIVED = 2,

        /// <summary>
        /// Conformation of STOP DT command received (server will no longer send or accept application layer messages)
        /// </summary>
        STOPDT_CON_RECEIVED = 3,

        /// <summary>
        /// The connect attempt has failed
        /// </summary>
        CONNECT_FAILED = 4
    }

    public enum CS104_ConState
    {
        STATE_IDLE = 0,
        STATE_INACTIVE = 1,
        STATE_ACTIVE = 2,
        STATE_WAITING_FOR_STARTDT_CON = 3,
        STATE_WAITING_FOR_STOPDT_CON = 4
    }

    /// <summary>
    /// Provides some Connection statistics.
    /// </summary>
    public class ConnectionStatistics
    {

        private int sentMsgCounter = 0;
        private int rcvdMsgCounter = 0;
        private int rcvdTestFrActCounter = 0;
        private int rcvdTestFrConCounter = 0;

        internal void Reset()
        {
            sentMsgCounter = 0;
            rcvdMsgCounter = 0;
            rcvdTestFrActCounter = 0;
            rcvdTestFrConCounter = 0;
        }

        /// <summary>
        /// Gets or sets the sent message counter.
        /// </summary>
        /// <value>The sent message counter.</value>
        public int SentMsgCounter
        {
            get
            {
                return sentMsgCounter;
            }
            internal set
            {
                sentMsgCounter = value;
            }
        }

        /// <summary>
        /// Gets or sets the received message counter.
        /// </summary>
        /// <value>The received message counter.</value>
        public int RcvdMsgCounter
        {
            get
            {
                return rcvdMsgCounter;
            }
            internal set
            {
                rcvdMsgCounter = value;
            }
        }

        /// <summary>
        /// Counter for the TEST_FR_ACT messages received.
        /// </summary>
        /// <value>The TEST_FR_ACT counter.</value>
        public int RcvdTestFrActCounter
        {
            get
            {
                return rcvdTestFrActCounter;
            }
            internal set
            {
                rcvdTestFrActCounter = value;
            }
        }

        /// <summary>
        /// Counter for the TEST_FR_CON messages received.
        /// </summary>
        /// <value>The TEST_FR_CON counter.</value>
        public int RcvdTestFrConCounter
        {
            get
            {
                return rcvdTestFrConCounter;
            }
            internal set
            {
                rcvdTestFrConCounter = value;
            }
        }
    }

    /// <summary>
    /// ASDU received handler.
    /// </summary>
    public delegate bool ASDUReceivedHandler(object parameter, ASDU asdu);

    /// <summary>
    /// Callback handler for connection events
    /// </summary>
    public delegate void ConnectionHandler(object parameter, ConnectionEvent connectionEvent);

    /// <summary>
    /// A single connection to a CS 104 (IEC 60870-5-104) server. Implements the \ref Master interface.
    /// </summary>
    public class Connection : Master
    {
        static byte[] STARTDT_ACT_MSG = new byte[] { 0x68, 0x04, 0x07, 0x00, 0x00, 0x00 };

        static byte[] STARTDT_CON_MSG = new byte[] { 0x68, 0x04, 0x0b, 0x00, 0x00, 0x00 };

        static byte[] STOPDT_ACT_MSG = new byte[] { 0x68, 0x04, 0x13, 0x00, 0x00, 0x00 };

        static byte[] STOPDT_CON_MSG = new byte[] { 0x68, 0x04, 0x23, 0x00, 0x00, 0x00 };

        static byte[] TESTFR_ACT_MSG = new byte[] { 0x68, 0x04, 0x43, 0x00, 0x00, 0x00 };

        static byte[] TESTFR_CON_MSG = new byte[] { 0x68, 0x04, 0x83, 0x00, 0x00, 0x00 };

        private int sendSequenceNumber;
        private int receiveSequenceNumber;

        private UInt64 uMessageTimeout = 0;

        /**********************************************/
        /* data structure for k-size sent ASDU buffer */
        private struct SentASDU
        {
            public long sentTime;
            // required for T1 timeout
            public int seqNo;
        }

        private int maxSentASDUs;
        /* maximum number of ASDU to be sent without confirmation - parameter k */
        private int oldestSentASDU = -1;
        /* index of oldest entry in k-buffer */
        private int newestSentASDU = -1;
        /* index of newest entry in k-buffer */
        private SentASDU[] sentASDUs = null;
        /* the k-buffer */

        /**********************************************/

        CS104_ConState conState;

        private bool checkSequenceNumbers = true;

        private Queue<ASDU> waitingToBeSent = null;
        private bool useSendMessageQueue = true;

        private UInt64 nextT3Timeout;
        private int outStandingTestFRConMessages = 0;

        private Thread workerThread = null;

        private int unconfirmedReceivedIMessages;
        /* number of unconfirmed messages received */

        /* T2 timeout handling */
        private long lastConfirmationTime;
        /* timestamp when the last confirmation message was sent */
        private bool timeoutT2Triggered = false;

        private Socket socket = null;
        private Stream netStream = null;
        private TlsSecurityInformation tlsSecInfo = null;

        private bool autostart = true;

        private FileClient fileClient = null;

        private string hostname;
        protected int tcpPort;

        private bool running = false;
        private bool connecting = false;
        private bool socketError;
        private SocketException lastException;

        private static int connectionCounter = 0;
        private int connectionID;

        private APCIParameters apciParameters;
        private ApplicationLayerParameters alParameters;

        private string localIpAddress = null;
        private int localTcpPort = 0;

        /// <summary>
        /// Set the local IP address for the local connection endpoint
        /// </summary>
        public string LocalIpAddress
        {
            get => localIpAddress;
            set => localIpAddress = value;
        }

        /// <summary>
        /// Set the TCP (source) port for the local connection endpoint (0 to automatically select a source port)
        /// </summary>
        public int LocalTcpPort
        {
            get => localTcpPort;
            set => localTcpPort = value;
        }

        /// <summary>
        /// Gets or sets a value indicating whether this <see cref="lib60870.Connection"/> use send message queue.
        /// </summary>
        /// <description>
        /// If <c>true</c> the Connection stores the ASDUs to be sent in a Queue when the connection cannot send
        /// ASDUs. This is the case when the counterpart (slave/server) is (temporarily) not able to handle new message,
        /// or the slave did not confirm the reception of the ASDUs for other reasons. If <c>false</c> the ASDU will be 
        /// ignored and a <see cref="lib60870.ConnectionException"/> will be thrown in this case.
        /// </description>
        /// <value><c>true</c> if use send message queue; otherwise, <c>false</c>.</value>
        public bool UseSendMessageQueue
        {
            get
            {
                return useSendMessageQueue;
            }
            set
            {
                useSendMessageQueue = value;
            }
        }

        /// <summary>
        /// Gets or sets the send sequence number N(S). WARNING: For test purposes only! Do net set
        /// in real application!
        /// </summary>
        /// <value>The send sequence number N(S)</value>
        public int SendSequenceNumber
        {
            get
            {
                return sendSequenceNumber;
            }
            set
            {
                sendSequenceNumber = value;
            }
        }

        protected bool CheckSequenceNumbers
        {
            get
            {
                return checkSequenceNumbers;
            }
            set
            {
                checkSequenceNumbers = value;
            }
        }

        /// <summary>
        /// Gets or sets the receive sequence number N(R). WARNING: For test purposes only! Do net set
        /// in real application!
        /// </summary>
        /// <value>The receive sequence number N(R)</value>
        public int ReceiveSequenceNumber
        {
            get
            {
                return receiveSequenceNumber;
            }
            set
            {
                receiveSequenceNumber = value;
            }
        }

        /// <summary>
        /// Gets or sets a value indicating whether this <see cref="lib60870.Connection"/> is automatically sends
        /// a STARTDT_ACT message on startup.
        /// </summary>
        /// <value><c>true</c> to send STARTDT_ACT message on startup; otherwise, <c>false</c>.</value>
        public bool Autostart
        {
            get
            {
                return autostart;
            }
            set
            {
                autostart = value;
            }
        }

        private void DebugLog(string message)
        {
            if (debugOutput)
                Console.WriteLine("CS104 MASTER CONNECTION " + connectionID + ": " + message);
        }

        private ConnectionStatistics statistics = new ConnectionStatistics();

        /// <summary>
        /// Resets the connection state by clearing sequence numbers, message counters, 
        /// and connection-specific flags. This method is used to initialize or reset 
        /// the connection before establishing a new session or after a connection is lost.
        /// </summary>
        private void ResetConnection()
        {
            sendSequenceNumber = 0;
            receiveSequenceNumber = 0;
            unconfirmedReceivedIMessages = 0;
            lastConfirmationTime = System.Int64.MaxValue;
            timeoutT2Triggered = false;
            outStandingTestFRConMessages = 0;

            uMessageTimeout = 0;

            socketError = false;
            lastException = null;

            maxSentASDUs = apciParameters.K;
            oldestSentASDU = -1;
            newestSentASDU = -1;
            sentASDUs = new SentASDU[maxSentASDUs];

            if (useSendMessageQueue)
                waitingToBeSent = new Queue<ASDU>();

            conState = CS104_ConState.STATE_IDLE;

            statistics.Reset();
        }

        private int connectTimeoutInMs = 1000;
        private int receiveTimeoutInMs = 1000; /* maximum allowed time between SOF byte and last message byte */

        public ApplicationLayerParameters Parameters
        {
            get
            {
                return alParameters;
            }
        }

        private ASDUReceivedHandler asduReceivedHandler = null;
        private object asduReceivedHandlerParameter = null;

        private ConnectionHandler connectionHandler = null;
        private object connectionHandlerParameter = null;

        private RawMessageHandler recvRawMessageHandler = null;
        private object recvRawMessageHandlerParameter = null;

        private RawMessageHandler sentMessageHandler = null;
        private object sentMessageHandlerParameter = null;

        /// <summary>
        /// Sends an S-Message to the remote device. 
        /// This message contains a sequence number and is used for communication control.
        /// </summary>
        private void SendSMessage()
        {
            byte[] msg = new byte[6];

            msg[0] = 0x68;
            msg[1] = 0x04;
            msg[2] = 0x01;
            msg[3] = 0;
            msg[4] = (byte)((receiveSequenceNumber % 128) * 2);
            msg[5] = (byte)(receiveSequenceNumber / 128);

            netStream.Write(msg, 0, msg.Length);

            statistics.SentMsgCounter++;

            if (sentMessageHandler != null)
            {
                sentMessageHandler(sentMessageHandlerParameter, msg, 6);
            }
        }

        /// <summary>
        /// Checks the validity of a received sequence number.
        /// </summary>
        /// <param name="seqNo">The sequence number to be validated.</param>
        /// <returns>
        /// Returns true if the sequence number is valid, false otherwise.
        /// </returns>
        private bool CheckSequenceNumber(int seqNo)
        {
            if (checkSequenceNumbers)
            {
                lock (sentASDUs)
                {
                    /* check if received sequence number is valid */

                    bool seqNoIsValid = false;
                    bool counterOverflowDetected = false;
                    int oldestValidSeqNo = -1;

                    if (oldestSentASDU == -1)
                    { /* if k-Buffer is empty */
                        if (seqNo == sendSequenceNumber)
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

                        /* check if confirmed message was already removed from list */
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
            }

            return true;
        }

        /// <summary>
        /// Checks if the send buffer is full, i.e., if there are no more available spaces 
        /// to add new sent messages in the circular buffer.
        /// </summary>
        /// <returns>
        /// Returns true if the send buffer is full, otherwise false.
        /// </returns>
        private bool IsSentBufferFull()
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
        /// Sends an I-Message (Information message) to the remote device.
        /// </summary>
        /// <param name="asdu">The ASDU (Application Service Data Unit) to be sent.</param>
        /// <returns>
        /// Returns the sequence number used for the I-Message after it has been sent.
        /// </returns>
        private int SendIMessage(ASDU asdu)
        {
            BufferFrame frame = new BufferFrame(new byte[260], 6); /* reserve space for ACPI */
            asdu.Encode(frame, alParameters);

            byte[] buffer = frame.GetBuffer();

            int msgSize = frame.GetMsgSize(); /* ACPI + ASDU */

            buffer[0] = 0x68;

            /* set size field */
            buffer[1] = (byte)(msgSize - 2);

            buffer[2] = (byte)((sendSequenceNumber % 128) * 2);
            buffer[3] = (byte)(sendSequenceNumber / 128);

            buffer[4] = (byte)((receiveSequenceNumber % 128) * 2);
            buffer[5] = (byte)(receiveSequenceNumber / 128);

            if (running)
            {
                netStream.Write(buffer, 0, msgSize);

                sendSequenceNumber = (sendSequenceNumber + 1) % 32768;
                statistics.SentMsgCounter++;

                unconfirmedReceivedIMessages = 0;
                timeoutT2Triggered = false;

                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, buffer, msgSize);
                }

                return sendSequenceNumber;
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Prints the contents of the k-buffer, showing the sequence numbers and sent times of the messages.
        /// </summary>
        private void PrintSendBuffer()
        {
            if (oldestSentASDU != -1)
            {
                int currentIndex = oldestSentASDU;

                int nextIndex = 0;

                DebugLog("------k-buffer------");

                do
                {
                    DebugLog(currentIndex + " : S " + sentASDUs[currentIndex].seqNo + " : time " +
                        sentASDUs[currentIndex].sentTime);

                    if (currentIndex == newestSentASDU)
                        nextIndex = -1;

                    currentIndex = (currentIndex + 1) % maxSentASDUs;

                } while (nextIndex != -1);

                DebugLog("--------------------");
            }
        }

        /// <summary>
        /// Sends an I-Message and updates the k-buffer with the new ASDU (Application Service Data Unit).
        /// </summary>
        /// <param name="asdu">The ASDU to be sent.</param>
        private void SendIMessageAndUpdateSentASDUs(ASDU asdu)
        {
            lock (sentASDUs)
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

                sentASDUs[currentIndex].seqNo = SendIMessage(asdu);
                sentASDUs[currentIndex].sentTime = SystemUtils.currentTimeMillis();

                newestSentASDU = currentIndex;

                PrintSendBuffer();
            }
        }

        /// <summary>
        /// Sends the next ASDU in the waiting queue if the k-buffer is not full.
        /// </summary>
        /// <returns>
        /// Returns `true` if an ASDU was sent, otherwise `false` if no ASDU was sent.
        /// </returns>
        private bool SendNextWaitingASDU()
        {
            bool sentAsdu = false;

            if (running == false)
                throw new ConnectionException("connection lost");

            try
            {
                lock (waitingToBeSent)
                {

                    while (waitingToBeSent.Count > 0)
                    {

                        if (IsSentBufferFull() == true)
                            break;

                        ASDU asdu = waitingToBeSent.Dequeue();

                        if (asdu != null)
                        {
                            SendIMessageAndUpdateSentASDUs(asdu);
                            sentAsdu = true;
                        }
                        else
                            break;

                    }
                }
            }
            catch (Exception)
            {
                running = false;
                throw new ConnectionException("connection lost");
            }

            return sentAsdu;
        }

        /// <summary>
        /// Sends an ASDU either immediately or by queuing it for later sending.
        /// </summary>
        /// <param name="asdu">The ASDU to be sent.</param>
        private void SendASDUInternal(ASDU asdu)
        {
            lock (socket)
            {
                if (running == false)
                    throw new ConnectionException("not connected", new SocketException(10057));

                if (useSendMessageQueue)
                {
                    lock (waitingToBeSent)
                    {
                        waitingToBeSent.Enqueue(asdu);
                    }

                    SendNextWaitingASDU();
                }
                else
                {

                    if (IsSentBufferFull())
                        throw new ConnectionException("Flow control congestion. Try again later.");

                    SendIMessageAndUpdateSentASDUs(asdu);
                }
            }
        }

        /// <summary>
        /// Sets up the connection parameters for the communication session.
        /// </summary>
        /// <param name="hostname">The hostname or IP address of the remote server.</param>
        /// <param name="apciParameters">The APCI (Application Protocol Control Information) parameters for the connection.</param>
        /// <param name="alParameters">The application layer parameters for the connection.</param>
        /// <param name="tcpPort">The TCP port number for the connection.</param>
        private void Setup(string hostname, APCIParameters apciParameters, ApplicationLayerParameters alParameters, int tcpPort)
        {
            this.hostname = hostname;
            this.alParameters = alParameters;
            this.apciParameters = apciParameters;
            this.tcpPort = tcpPort;
            connectTimeoutInMs = apciParameters.T0 * 1000;

            connectionCounter++;
            connectionID = connectionCounter;
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS104.Connection"/> class.
        /// </summary>
        /// <param name="hostname">hostname of IP address of the CS 104 server</param>
        /// <param name="tcpPort">TCP port of the CS 104 server</param>
        public Connection(string hostname, int tcpPort = 2404)
        {
            Setup(hostname, new APCIParameters(), new ApplicationLayerParameters(), tcpPort);
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS104.Connection"/> class.
        /// </summary>
        /// <param name="hostname">hostname of IP address of the CS 104 server</param>
        /// <param name="apciParameters">APCI parameters.</param>
        /// <param name="alParameters">application layer parameters.</param>
        public Connection(string hostname, APCIParameters apciParameters, ApplicationLayerParameters alParameters)
        {
            Setup(hostname, apciParameters.Clone(), alParameters.Clone(), 2404);
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS104.Connection"/> class.
        /// </summary>
        /// <param name="hostname">hostname of IP address of the CS 104 server</param>
        /// <param name="tcpPort">TCP port of the CS 104 server</param>
        /// <param name="apciParameters">APCI parameters.</param>
        /// <param name="alParameters">application layer parameters.</param>
        public Connection(string hostname, int tcpPort, APCIParameters apciParameters, ApplicationLayerParameters alParameters)
        {
            Setup(hostname, apciParameters.Clone(), alParameters.Clone(), tcpPort);
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS104.Connection"/> class using TLS.
        /// </summary>
        /// <param name="hostname">hostname of IP address of the CS 104 server</param>
        /// <param name="tlsInfo">TLS setup</param>
        /// <param name="tcpPort">TCP port of the CS 104 server</param>
        public Connection(string hostname, TlsSecurityInformation tlsInfo, int tcpPort = 19998)
        {
            Setup(hostname, new APCIParameters(), new ApplicationLayerParameters(), tcpPort);
            tlsSecInfo = tlsInfo;
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS104.Connection"/> class using TLS.
        /// </summary>
        /// <param name="hostname">hostname of IP address of the CS 104 server</param>
        /// <param name="tcpPort">TCP port of the CS 104 server</param>
        /// <param name="apciParameters">APCI parameters.</param>
        /// <param name="alParameters">application layer parameters.</param>
        /// <param name="tlsInfo">TLS setup</param>
        public Connection(string hostname, int tcpPort, APCIParameters apciParameters, ApplicationLayerParameters alParameters, TlsSecurityInformation tlsInfo)
        {
            Setup(hostname, apciParameters.Clone(), alParameters.Clone(), tcpPort);
            tlsSecInfo = tlsInfo;
        }

        /// <summary>
        /// Set the security parameters for TLS
        /// </summary>
        /// <param name="securityInfo">Security info.</param>
        public void SetTlsSecurity(TlsSecurityInformation securityInfo)
        {
            tlsSecInfo = securityInfo;
        }

        /// <summary>
        /// Gets the conenction statistics.
        /// </summary>
        /// <returns>The connection statistics.</returns>
        public ConnectionStatistics GetStatistics()
        {
            return statistics;
        }

        /// <summary>
        /// Sets the connect timeout
        /// </summary>
        /// <param name="millies">timeout value in milliseconds (ms)</param>
        public void SetConnectTimeout(int millies)
        {
            connectTimeoutInMs = millies;
        }

        /// <summary>
        /// Timeout for connection establishment in milliseconds (ms)
        /// </summary>
        public int ConnectTimeout
        {
            get
            {
                return connectTimeoutInMs;
            }
            set
            {
                connectTimeoutInMs = value;
            }
        }

        /// <summary>
        /// Maximum allowed time for receiving a single message
        /// </summary>
        public int ReceiveTimeout
        {
            get
            {
                return receiveTimeoutInMs;
            }
            set
            {
                receiveTimeoutInMs = value;
            }
        }

        /// <summary>
        /// Sends the interrogation command.
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qoi">Qualifier of interrogation (20 = station interrogation)</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendInterrogationCommand(CauseOfTransmission cot, int ca, byte qoi)
        {
            ASDU asdu = new ASDU(alParameters, cot, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new InterrogationCommand(0, qoi));

            SendASDUInternal(asdu);
        }

        /// <summary>
        /// Sends the counter interrogation command (C_CI_NA_1 typeID: 101)
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qcc">Qualifier of counter interrogation command</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendCounterInterrogationCommand(CauseOfTransmission cot, int ca, byte qcc)
        {
            ASDU asdu = new ASDU(alParameters, cot, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new CounterInterrogationCommand(0, qcc));

            SendASDUInternal(asdu);
        }

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
        public override void SendReadCommand(int ca, int ioa)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.REQUEST, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new ReadCommand(ioa));

            SendASDUInternal(asdu);
        }

        /// <summary>
        /// Sends a clock synchronization command (C_CS_NA_1 typeID: 103).
        /// </summary>
        /// <param name="ca">Common address</param>
        /// <param name="time">the new time to set</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendClockSyncCommand(int ca, CP56Time2a time)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new ClockSynchronizationCommand(0, time));

            SendASDUInternal(asdu);
        }

        /// <summary>
        /// Sends a test command (C_TS_NA_1 typeID: 104).
        /// </summary>
        /// 
        /// Not required and supported by IEC 60870-5-104. 
        /// 
        /// <param name="ca">Common address</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendTestCommand(int ca)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new TestCommand());

            SendASDUInternal(asdu);
        }

        /// <summary>
        /// Sends a test command with CP56Time2a time (C_TS_TA_1 typeID: 107).
        /// </summary>
        /// <param name="ca">Common address</param>
        /// <param name="tsc">test sequence number</param>
        /// <param name="time">test timestamp</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendTestCommandWithCP56Time2a(int ca, ushort tsc, CP56Time2a time)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new TestCommandWithCP56Time2a(tsc, time));

            SendASDUInternal(asdu);
        }

        /// <summary>
        /// Sends a reset process command (C_RP_NA_1 typeID: 105).
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="qrp">Qualifier of reset process command</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendResetProcessCommand(CauseOfTransmission cot, int ca, byte qrp)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new ResetProcessCommand(0, qrp));

            SendASDUInternal(asdu);
        }


        /// <summary>
        /// Sends a delay acquisition command (C_CD_NA_1 typeID: 106).
        /// </summary>
        /// <param name="cot">Cause of transmission</param>
        /// <param name="ca">Common address</param>
        /// <param name="delay">delay for acquisition</param>
        /// <exception cref="ConnectionException">description</exception>
        public override void SendDelayAcquisitionCommand(CauseOfTransmission cot, int ca, CP16Time2a delay)
        {
            ASDU asdu = new ASDU(alParameters, CauseOfTransmission.ACTIVATION, false, false, (byte)alParameters.OA, ca, false);

            asdu.AddInformationObject(new DelayAcquisitionCommand(0, delay));

            SendASDUInternal(asdu);
        }

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
        public override void SendControlCommand(CauseOfTransmission cot, int ca, InformationObject sc)
        {

            ASDU controlCommand = new ASDU(alParameters, cot, false, false, (byte)alParameters.OA, ca, false);

            controlCommand.AddInformationObject(sc);

            SendASDUInternal(controlCommand);
        }

        /// <summary>
        /// Sends an arbitrary ASDU to the connected slave
        /// </summary>
        /// <param name="asdu">The ASDU to send</param>
        public override void SendASDU(ASDU asdu)
        {
            SendASDUInternal(asdu);
        }

        public override ApplicationLayerParameters GetApplicationLayerParameters()
        {
            return alParameters;
        }

        /// <summary>
        /// Start data transmission on this connection
        /// </summary>
        public void SendStartDT()
        {
            if (running)
            {
                try
                {
                    conState = CS104_ConState.STATE_WAITING_FOR_STARTDT_CON;
                    netStream.Write(STARTDT_ACT_MSG, 0, STARTDT_ACT_MSG.Length);
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;

                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, STARTDT_ACT_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Stop data transmission on this connection
        /// </summary>
        public void SendStopDT()
        {
            if (running)
            {
                try
                {
                    if (unconfirmedReceivedIMessages > 0)
                    {
                        /* confirm all unconfirmed messages before stopping the connection */

                        lastConfirmationTime = SystemUtils.currentTimeMillis();

                        unconfirmedReceivedIMessages = 0;
                        timeoutT2Triggered = false;

                        SendSMessage();
                    }

                    netStream.Write(STOPDT_ACT_MSG, 0, STOPDT_ACT_MSG.Length);

                    conState = CS104_ConState.STATE_WAITING_FOR_STOPDT_CON;
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;
                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, STOPDT_ACT_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Start application layer data transmission on this connection
        /// </summary>
        protected void SendStartDT_CON()
        {
            if (running)
            {
                try
                {
                    netStream.Write(STARTDT_CON_MSG, 0, STARTDT_CON_MSG.Length);
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;
                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, STARTDT_CON_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Sends a STOPDT_CON message to the remote server if the connection is active.
        /// </summary>
        protected void SendStopDT_CON()
        {
            if (running)
            {
                try
                {
                    netStream.Write(STOPDT_CON_MSG, 0, STOPDT_CON_MSG.Length);
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;
                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, STOPDT_CON_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Sends a TESTFR_ACT message to the remote server if the connection is active.
        /// </summary>
        protected void SendTestFR_ACT()
        {
            if (running)
            {
                try
                {
                    netStream.Write(TESTFR_ACT_MSG, 0, TESTFR_ACT_MSG.Length);
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;
                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, TESTFR_ACT_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Sends a TESTFR_CON message to the remote server if the connection is active.
        /// </summary>
        protected void SendTestFR_CON()
        {
            if (running)
            {
                try
                {
                    netStream.Write(TESTFR_CON_MSG, 0, TESTFR_CON_MSG.Length);
                }
                catch (Exception ex)
                {
                    throw new ConnectionException("Failed to write to socket", ex);
                }

                statistics.SentMsgCounter++;
                if (sentMessageHandler != null)
                {
                    sentMessageHandler(sentMessageHandlerParameter, TESTFR_CON_MSG, 6);
                }
            }
            else
            {
                if (lastException != null)
                    throw new ConnectionException(lastException.Message, lastException);
                else
                    throw new ConnectionException("not connected", new SocketException(10057));
            }
        }

        /// <summary>
        /// Connect this instance.
        /// </summary>
        /// 
        /// The function will throw a SocketException if the connection attempt is rejected or timed out.
        /// <exception cref="ConnectionException">description</exception>
        public void Connect()
        {
            ConnectAsync();

            while ((running == false) && (socketError == false))
            {
                Thread.Sleep(1);
            }

            if (socketError)
                throw new ConnectionException(lastException.Message, lastException);
        }

        /// <summary>
        /// Resets the T3 timeout based on the current system time and the T3 parameter from the APCI configuration.
        /// </summary>
        private void ResetT3Timeout()
        {
            nextT3Timeout = (UInt64)SystemUtils.currentTimeMillis() + (UInt64)(apciParameters.T3 * 1000);
        }

        /// <summary>
        /// Connects to the server (outstation). This is a non-blocking call. Before using the connection
        /// you have to check if the connection is already connected and running.
        /// </summary>
        /// <exception cref="ConnectionException">description</exception>
        public void ConnectAsync()
        {
            if ((running == false) && (connecting == false))
            {
                ResetConnection();

                ResetT3Timeout();

                workerThread = new Thread(HandleConnection);

                workerThread.Start();
            }
            else
            {
                if (running)
                    throw new ConnectionException("already connected", new SocketException(10056)); /* WSAEISCONN - Socket is already connected */
                else
                    throw new ConnectionException("already connecting", new SocketException(10037)); /* WSAEALREADY - Operation already in progress */

            }
        }

        private int readState = 0; /* 0 - idle, 1 - start received, 2 - reading remaining bytes */
        private int currentReadPos = 0;
        private int currentReadMsgLength = 0;
        private int remainingReadLength = 0;
        private long currentReadTimeout = 0;

        /// <summary>
        /// Receives a message from the network stream and processes it byte by byte.
        /// </summary>
        /// <param name="buffer">The byte array to store the received data.</param>
        /// <returns>
        /// Returns the total number of bytes read if a message is successfully received.
        /// Returns -1 if there is a timeout or invalid data (e.g., missing SOF or incorrect length).
        /// Returns 0 if the message is incomplete and the method needs to be called again.
        /// </returns>
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
                    if (netStream.Read(buffer, 0, 1) != 1)
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
                    if (netStream.Read(buffer, 1, 1) != 1)
                        return 0;

                    currentReadMsgLength = buffer[1];
                    remainingReadLength = currentReadMsgLength;
                    currentReadPos = 2;

                    readState = 2;
                }

                if (readState == 2)
                {
                    int readLength = netStream.Read(buffer, currentReadPos, remainingReadLength);

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
                    currentReadTimeout = SystemUtils.currentTimeMillis() + receiveTimeoutInMs;
                }
            }

            return 0;
        }

        /// <summary>
        /// Checks if the confirmation timeout (T2) has elapsed since the last confirmation time.
        /// </summary>
        /// <param name="currentTime">The current time in milliseconds.</param>
        /// <returns>
        /// Returns `true` if the confirmation timeout (T2) has elapsed, otherwise returns `false`.
        /// </returns>
        private bool checkConfirmTimeout(long currentTime)
        {
            if ((currentTime - lastConfirmationTime) >= (apciParameters.T2 * 1000))
                return true;
            else
                return false;
        }

        /// <summary>
        /// Processes an incoming message, checks its validity, and handles different types of frames.
        /// </summary>
        /// <param name="buffer">The byte array containing the received message.</param>
        /// <param name="msgSize">The size of the received message in bytes.</param>
        /// <returns>
        /// Returns `true` if the message is valid and successfully processed.
        /// Returns `false` if the message is invalid or an error occurs during processing.
        /// </returns>
        private bool checkMessage(byte[] buffer, int msgSize)
        {
            long currentTime = SystemUtils.currentTimeMillis();

            if ((buffer[2] & 1) == 0)
            { /* I format frame */

                if (timeoutT2Triggered == false)
                {
                    timeoutT2Triggered = true;
                    lastConfirmationTime = currentTime; /* start timeout T2 */
                }

                if (msgSize < 7)
                {
                    DebugLog("I msg too small!");
                    return false;
                }

                int frameSendSequenceNumber = ((buffer[3] * 0x100) + (buffer[2] & 0xfe)) / 2;
                int frameRecvSequenceNumber = ((buffer[5] * 0x100) + (buffer[4] & 0xfe)) / 2;

                DebugLog("Received I frame: N(S) = " + frameSendSequenceNumber + " N(R) = " + frameRecvSequenceNumber);

                /* check the receive sequence number N(R) - connection will be closed on an unexpected value */
                if (frameSendSequenceNumber != receiveSequenceNumber)
                {
                    DebugLog("Sequence error: Close connection!");
                    return false;
                }

                if (CheckSequenceNumber(frameRecvSequenceNumber) == false)
                    return false;

                receiveSequenceNumber = (receiveSequenceNumber + 1) % 32768;
                unconfirmedReceivedIMessages++;

                try
                {
                    ASDU asdu = new ASDU(alParameters, buffer, 6, msgSize);

                    bool messageHandled = false;

                    if (fileClient != null)
                        messageHandled = fileClient.HandleFileAsdu(asdu);

                    if (messageHandled == false)
                    {

                        if (asduReceivedHandler != null)
                            asduReceivedHandler(asduReceivedHandlerParameter, asdu);

                    }
                }
                catch (ASDUParsingException e)
                {
                    DebugLog("ASDU parsing failed: " + e.Message);
                    return false;
                }

            }
            else if ((buffer[2] & 0x03) == 0x01)
            { /* S format frame */
                int seqNo = (buffer[4] + buffer[5] * 0x100) / 2;

                DebugLog("Recv S(" + seqNo + ") (own sendcounter = " + sendSequenceNumber + ")");

                if (CheckSequenceNumber(seqNo) == false)
                    return false;
            }
            else if ((buffer[2] & 0x03) == 0x03)
            { /* U format frame */

                uMessageTimeout = 0;

                if (buffer[2] == 0x43)
                { 
                    /* Check for TESTFR_ACT message */
                    statistics.RcvdTestFrActCounter++;
                    DebugLog("RCVD TESTFR_ACT");
                    DebugLog("SEND TESTFR_CON");

                    netStream.Write(TESTFR_CON_MSG, 0, TESTFR_CON_MSG.Length);

                    statistics.SentMsgCounter++;
                    if (sentMessageHandler != null)
                    {
                        sentMessageHandler(sentMessageHandlerParameter, TESTFR_CON_MSG, 6);
                    }

                }
                else if (buffer[2] == 0x83)
                { /* TESTFR_CON */
                    DebugLog("RCVD TESTFR_CON");
                    statistics.RcvdTestFrConCounter++;
                    outStandingTestFRConMessages = 0;
                }
                else if (buffer[2] == 0x07)
                { /* STARTDT ACT */
                    DebugLog("RCVD STARTDT_ACT");

                    netStream.Write(STARTDT_CON_MSG, 0, STARTDT_CON_MSG.Length);

                    statistics.SentMsgCounter++;
                    if (sentMessageHandler != null)
                    {
                        sentMessageHandler(sentMessageHandlerParameter, STARTDT_CON_MSG, 6);
                    }

                    conState = CS104_ConState.STATE_ACTIVE;
                }
                else if (buffer[2] == 0x0b)
                { /* STARTDT_CON */
                    DebugLog("RCVD STARTDT_CON");

                    if (connectionHandler != null)
                        connectionHandler(connectionHandlerParameter, ConnectionEvent.STARTDT_CON_RECEIVED);

                    conState = CS104_ConState.STATE_ACTIVE;

                }
                else if (buffer[2] == 0x23)
                { /* STOPDT_CON */
                    DebugLog("RCVD STOPDT_CON");

                    if (connectionHandler != null)
                        connectionHandler(connectionHandlerParameter, ConnectionEvent.STOPDT_CON_RECEIVED);

                    conState = CS104_ConState.STATE_INACTIVE;
                }

            }
            else
            {
                DebugLog("Unknown message type");
                return false;
            }

            ResetT3Timeout();

            return true;
        }

        /// <summary>
        /// Checks the connection status by attempting to send a dummy byte to the socket.
        /// </summary>
        /// <returns>
        /// Returns `true` if the connection is active (i.e., the socket is still connected).
        /// Returns `false` if the socket is disconnected or an error occurs.
        /// </returns>
        private bool isConnected()
        {
            try
            {
                byte[] tmp = new byte[1];

                socket.Send(tmp, 0, 0);

                return true;
            }
            catch (SocketException e)
            {
                if (e.NativeErrorCode.Equals(10035))
                {
                    DebugLog("Still Connected, but the Send would block");
                    return true;
                }
                else
                {
                    DebugLog("Disconnected: error code " + e.NativeErrorCode);
                    return false;
                }
            }
        }

        /// <summary>
        /// Establishes a socket connection to the specified remote endpoint with a timeout. 
        /// If the connection cannot be established within the specified timeout, it throws a timeout exception.
        /// </summary>
        private void ConnectSocketWithTimeout()
        {
            IPAddress ipAddress;
            IPEndPoint remoteEP;

            try
            {
                ipAddress = IPAddress.Parse(hostname);
                remoteEP = new IPEndPoint(ipAddress, tcpPort);
            }
            catch (Exception)
            {
                throw new SocketException(87); /* wrong argument */
            }

            /* Create a TCP/IP  socket. */
            socket = new Socket(AddressFamily.InterNetwork,
                SocketType.Stream, ProtocolType.Tcp);

            socket.SetSocketOption(SocketOptionLevel.Socket, SocketOptionName.ReuseAddress, true);
            LingerOption lingerOption = new LingerOption(true, 0);
            socket.SetSocketOption(SocketOptionLevel.Socket, SocketOptionName.Linger, lingerOption);

            if (LocalIpAddress != null)
            {
                try
                {
                    socket.Bind(new IPEndPoint(IPAddress.Parse(localIpAddress), localTcpPort));
                }
                catch (Exception ex)
                {
                    Console.WriteLine(ex.Message);
                    throw new SocketException(87); /* wrong argument */
                }
            }

            var result = socket.BeginConnect(remoteEP, null, null);

            bool success = result.AsyncWaitHandle.WaitOne(connectTimeoutInMs, true);

            if (success)
            {
                try
                {
                    socket.EndConnect(result);
                    socket.NoDelay = true;
                    netStream = new NetworkStream(socket);
                }
                catch (ObjectDisposedException)
                {
                    socket = null;

                    DebugLog("ObjectDisposedException -> Connect canceled");

                    throw new SocketException(995); /* WSA_OPERATION_ABORTED */
                }
            }
            else
            {
                if (socket.Connected)
                {
                    try
                    {
                        socket.Shutdown(SocketShutdown.Receive);
                    }
                    catch (SocketException ex)
                    {
                        Console.WriteLine(ex.Message);
                    }

                    socket.Disconnect(true);
                }


                socket.Close(0);
                socket = null;

                throw new SocketException(10060); /* Connection timed out (WSAETiMEDOUT) */
            }
        }

        /// <summary>
        /// Handles various timeouts associated with the communication protocol, including T3, T1, and confirmation timeouts.
        /// The method checks for expired timeouts and takes appropriate actions, such as resending messages or throwing exceptions.
        /// </summary>
        /// <returns>
        /// - **true** if no timeouts or issues are detected, allowing normal processing to continue.
        /// - **false** if a timeout (such as T3) has occurred, indicating the need for further action (like message retransmission).
        /// </returns>
        private bool handleTimeouts()
        {
            UInt64 currentTime = (UInt64)SystemUtils.currentTimeMillis();

            if (currentTime > nextT3Timeout)
            {

                if (outStandingTestFRConMessages > 2)
                {
                    DebugLog("Timeout for TESTFR_CON message");

                    return false;
                }
                else
                {
                    netStream.Write(TESTFR_ACT_MSG, 0, TESTFR_ACT_MSG.Length);

                    statistics.SentMsgCounter++;
                    DebugLog("U message T3 timeout");
                    uMessageTimeout = currentTime + (UInt64)(apciParameters.T1 * 1000);
                    outStandingTestFRConMessages++;
                    ResetT3Timeout();
                    if (sentMessageHandler != null)
                    {
                        sentMessageHandler(sentMessageHandlerParameter, TESTFR_ACT_MSG, 6);
                    }
                }
            }

            if (unconfirmedReceivedIMessages > 0)
            {
                if (checkConfirmTimeout((long)currentTime))
                {
                    lastConfirmationTime = (long)currentTime;

                    unconfirmedReceivedIMessages = 0;
                    timeoutT2Triggered = false;

                    SendSMessage(); /* send confirmation message */
                }
            }

            if (uMessageTimeout != 0)
            {
                if (currentTime > uMessageTimeout)
                {
                    DebugLog("U message T1 timeout");
                    throw new SocketException(10060);
                }
            }

            /* check if counterpart confirmed I messages */
            lock (sentASDUs)
            {
                if (oldestSentASDU != -1)
                {

                    if (((long)currentTime - sentASDUs[oldestSentASDU].sentTime) >= (apciParameters.T1 * 1000))
                    {
                        return false;
                    }
                }
            }

            return true;
        }

        /// <summary>
        /// Compares two byte arrays to check if they are equal.
        /// </summary>
        /// <param name="array1">The first byte array to compare.</param>
        /// <param name="array2">The second byte array to compare.</param>
        /// <returns>
        /// - **true** if both byte arrays have the same length and the same content.
        /// - **false** if the byte arrays have different lengths or any byte at a corresponding index is different.
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
        /// Callback function for validating a server certificate in an SSL/TLS connection.
        /// </summary>
        /// <param name="sender">The source of the certificate validation request (typically the sender of the SSL/TLS connection).</param>
        /// <param name="certificate">The server's certificate to validate.</param>
        /// <param name="chain">The certificate chain containing the server's certificate and any intermediary certificates.</param>
        /// <param name="sslPolicyErrors">Any SSL policy errors encountered during the validation process.</param>
        /// <returns>
        /// - **true** if the certificate is valid according to the validation rules.
        /// - **false** if the certificate is invalid or fails validation based on specified rules.
        /// </returns>
        /// <remarks>
        private bool CertificateValidationCallback(Object sender, X509Certificate certificate, X509Chain chain, SslPolicyErrors sslPolicyErrors)
        {
            if (certificate != null)
            {

                if (tlsSecInfo.ChainValidation)
                {

                    X509Chain newChain = new X509Chain();

                    newChain.ChainPolicy.RevocationMode = X509RevocationMode.NoCheck;
                    newChain.ChainPolicy.RevocationFlag = X509RevocationFlag.ExcludeRoot;
                    newChain.ChainPolicy.VerificationFlags = X509VerificationFlags.AllowUnknownCertificateAuthority;
                    newChain.ChainPolicy.VerificationTime = DateTime.Now;

                    foreach (X509Certificate2 caCert in tlsSecInfo.CaCertificates)
                        newChain.ChainPolicy.ExtraStore.Add(caCert);

                    bool certificateStatus = newChain.Build(new X509Certificate2(certificate.GetRawCertData()));

                    if (certificateStatus == false)
                        return false;
                }

                if (tlsSecInfo.AllowOnlySpecificCertificates)
                {

                    foreach (X509Certificate2 allowedCert in tlsSecInfo.AllowedCertificates)
                    {
                        if (AreByteArraysEqual(allowedCert.GetCertHash(), certificate.GetCertHash()))
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
        /// Callback function for selecting a local certificate during an SSL/TLS handshake.
        /// </summary>
        /// <param name="sender">The source of the certificate selection request (typically the client in an SSL/TLS connection).</param>
        /// <param name="targetHost">The name of the server being connected to (host name or IP address).</param>
        /// <param name="localCertificates">The collection of local certificates available for selection.</param>
        /// <param name="remoteCertificate">The remote server's certificate being validated (not used in this implementation).</param>
        /// <param name="acceptableIssuers">A list of acceptable issuers for the certificate (not used in this implementation).</param>
        /// <returns>
        /// The local certificate to be used for the SSL/TLS connection. In this case, the first certificate in the collection.
        /// </returns>
        public X509Certificate LocalCertificateSelectionCallback(object sender, string targetHost, X509CertificateCollection localCertificates, X509Certificate remoteCertificate, string[] acceptableIssuers)
        {
            return localCertificates[0];
        }

        /// <summary>
        /// Establishes and maintains the connection to a remote device, handling socket connection, 
        /// TLS handshake, message reception, and timeouts. It processes incoming data, manages connection state,
        /// and ensures proper handling of message timeouts and unconfirmed messages.
        /// </summary>
        private void HandleConnection()
        {
            byte[] bytes = new byte[300];

            try
            {
                try
                {
                    connecting = true;

                    try
                    {
                        /* Connect to a remote device. */
                        ConnectSocketWithTimeout();

                        DebugLog("Socket connected to " + socket.RemoteEndPoint.ToString());

                        if (tlsSecInfo != null)
                        {
                            DebugLog("Setup TLS");

                            System.Net.ServicePointManager.SecurityProtocol = SecurityProtocolType.Tls | SecurityProtocolType.Tls11 | SecurityProtocolType.Tls12 | (SecurityProtocolType)12288 /* TLS 1.3 */;

                            RemoteCertificateValidationCallback validationCallback = CertificateValidationCallback;

                            if (tlsSecInfo.CertificateValidationCallback != null)
                                validationCallback = tlsSecInfo.CertificateValidationCallback;

                            SslStream sslStream = new SslStream(netStream, true, validationCallback, LocalCertificateSelectionCallback);

                            var clientCertificateCollection = new X509Certificate2Collection(tlsSecInfo.OwnCertificate);

                            try
                            {
                                string targetHostName = tlsSecInfo.TargetHostName;

                                if (targetHostName == null)
                                    targetHostName = "*";

                                System.Security.Authentication.SslProtocols tlsVersion = System.Security.Authentication.SslProtocols.None;

                                if (tlsSecInfo != null)
                                    tlsVersion = tlsSecInfo.TlsVersion;

                                DebugLog("Using TLS version: " + tlsVersion.ToString());

                                sslStream.AuthenticateAsClient(targetHostName, clientCertificateCollection, tlsVersion, false);
                            }
                            catch (IOException e)
                            {
                                string message;

                                if (e.GetBaseException() != null)
                                {
                                    message = e.GetBaseException().Message;
                                }
                                else
                                {
                                    message = e.Message;
                                }

                                DebugLog("TLS authentication error: " + message);

                                throw new SocketException(10060);
                            }
                            catch (System.Security.Authentication.AuthenticationException ex)
                            {
                                DebugLog("TLS authentication exception during connection setup: " + ex.Message);

                                throw new SocketException(10060);
                            }

                            if (sslStream.IsAuthenticated)
                            {
                                netStream = sslStream;
                            }
                            else
                            {
                                throw new SocketException(10060);
                            }

                        }

                        netStream.ReadTimeout = 50;

                        if (autostart)
                        {
                            netStream.Write(STARTDT_ACT_MSG, 0, STARTDT_ACT_MSG.Length);

                            statistics.SentMsgCounter++;
                        }

                        running = true;
                        socketError = false;
                        connecting = false;

                        if (connectionHandler != null)
                            connectionHandler(connectionHandlerParameter, ConnectionEvent.OPENED);

                    }
                    catch (SocketException se)
                    {
                        DebugLog("SocketException: " + se.ToString());

                        running = false;
                        socketError = true;
                        lastException = se;

                        if (connectionHandler != null)
                            connectionHandler(connectionHandlerParameter, ConnectionEvent.CONNECT_FAILED);
                    }

                    if (running)
                    {
                        conState = CS104_ConState.STATE_INACTIVE;

                        bool loopRunning = running;

                        while (loopRunning)
                        {

                            bool suspendThread = true;

                            try
                            {
                                /* Receive a message  from the remote device. */
                                int bytesRec = receiveMessage(bytes);

                                if (bytesRec > 0)
                                {

                                    DebugLog("RCVD: " + BitConverter.ToString(bytes, 0, bytesRec));

                                    statistics.RcvdMsgCounter++;

                                    bool handleMessage = true;

                                    CS104_ConState oldState = conState;

                                    if (recvRawMessageHandler != null)
                                        handleMessage = recvRawMessageHandler(recvRawMessageHandlerParameter, bytes, bytesRec);

                                    if (handleMessage)
                                    {
                                        if (checkMessage(bytes, bytesRec) == false)
                                        {
                                            /* close connection on error */
                                            loopRunning = false;
                                        }
                                    }

                                    CS104_ConState newState = conState;

                                    if ((newState != oldState) && connectionHandler != null)
                                    {
                                        if (newState == CS104_ConState.STATE_ACTIVE)
                                            connectionHandler(connectionHandlerParameter, ConnectionEvent.STARTDT_CON_RECEIVED);
                                        else if (newState == CS104_ConState.STATE_INACTIVE)
                                            connectionHandler(connectionHandlerParameter, ConnectionEvent.STOPDT_CON_RECEIVED);
                                    }

                                    if (unconfirmedReceivedIMessages >= apciParameters.W || conState == CS104_ConState.STATE_WAITING_FOR_STOPDT_CON)
                                    {
                                        lastConfirmationTime = SystemUtils.currentTimeMillis();

                                        unconfirmedReceivedIMessages = 0;
                                        timeoutT2Triggered = false;

                                        SendSMessage();
                                    }

                                    suspendThread = false;
                                }
                                else if (bytesRec == -1)
                                    loopRunning = false;


                                if (handleTimeouts() == false)
                                    loopRunning = false;

                                if (fileClient != null)
                                    fileClient.HandleFileService();

                                if (isConnected() == false)
                                    loopRunning = false;

                                if (running == false)
                                    loopRunning = false;

                                if (useSendMessageQueue)
                                {
                                    if (SendNextWaitingASDU() == true)
                                        suspendThread = false;
                                }

                                if (suspendThread)
                                    Thread.Sleep(1);

                            }
                            catch (SocketException)
                            {
                                loopRunning = false;
                            }
                            catch (System.IO.IOException e)
                            {
                                DebugLog("IOException: " + e.ToString());
                                loopRunning = false;
                            }
                            catch (ConnectionException)
                            {
                                loopRunning = false;
                            }
                        }

                        DebugLog("CLOSE CONNECTION!");


                        if (unconfirmedReceivedIMessages > 0)
                        {
                            /* confirm all unconfirmed messages before stopping the connection */

                            lastConfirmationTime = SystemUtils.currentTimeMillis();

                            unconfirmedReceivedIMessages = 0;
                            timeoutT2Triggered = false;

                            SendSMessage();
                        }


                        running = false;
                        socketError = true;

                        if (socket.Connected)
                        {
                            try
                            {
                                socket.Shutdown(SocketShutdown.Receive);
                            }
                            catch (SocketException ex)
                            {
                                Console.WriteLine(ex.Message);
                            }

                            socket.Disconnect(true);
                        }


                        socket.Close(0);

                        netStream.Dispose();

                        if (connectionHandler != null)
                            connectionHandler(connectionHandlerParameter, ConnectionEvent.CLOSED);
                    }

                }
                catch (ArgumentNullException ane)
                {
                    connecting = false;
                    DebugLog("ArgumentNullException: " + ane.ToString());
                }
                catch (SocketException se)
                {
                    DebugLog("SocketException: " + se.ToString());
                }
                catch (ConnectionException se)
                {
                    DebugLog("ConnectionException: " + se.ToString());
                }
                catch (Exception e)
                {
                    DebugLog("Unexpected exception: " + e.ToString());
                }

            }
            catch (ConnectionException se)
            {
                DebugLog("ConnectionException: " + se.ToString());
            }
            catch (Exception e)
            {
                DebugLog(e.ToString());
            }

            conState = CS104_ConState.STATE_IDLE;

            running = false;
            connecting = false;
        }

        /// <summary>
        /// Gets a value indicating whether this <see cref="T:lib60870.CS104.Connection"/> is running(connected).
        /// </summary>
        /// <value><c>true</c> if is running/connected; otherwise, <c>false</c>.</value>
        public bool IsRunning
        {
            get
            {
                return running;
            }
        }

        public void Cancel()
        {
            if (socket != null)
            {
                if (socket.Connected)
                {
                    try
                    {
                        socket.Shutdown(SocketShutdown.Receive);
                    }
                    catch (SocketException ex)
                    {
                        Console.WriteLine(ex.Message);
                    }

                    socket.Disconnect(true);

                }


                socket.Close(0);
            }

        }

        public void Close()
        {
            if (running)
            {
                running = false;
                workerThread.Join();
            }
        }

        /// <summary>
        /// Set the ASDUReceivedHandler. This handler is invoked whenever a new ASDU is received
        /// </summary>
        /// <param name="handler">the handler to be called</param>
        /// <param name="parameter">user provided parameter that is passed to the handler</param>
        public void SetASDUReceivedHandler(ASDUReceivedHandler handler, object parameter)
        {
            asduReceivedHandler = handler;
            asduReceivedHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets the connection handler. The connection handler is called when
        /// the connection is established or closed
        /// </summary>
        /// <param name="handler">the handler to be called</param>
        /// <param name="parameter">user provided parameter that is passed to the handler</param>
        public void SetConnectionHandler(ConnectionHandler handler, object parameter)
        {
            connectionHandler = handler;
            connectionHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets the raw message handler. This is a callback to intercept raw messages received.
        /// </summary>
        /// <param name="handler">Handler/delegate that will be invoked when a message is received</param>
        /// <param name="parameter">will be passed to the delegate</param>
        public override void SetReceivedRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            recvRawMessageHandler = handler;
            recvRawMessageHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets the sent message handler. This is a callback to intercept the sent raw messages
        /// </summary>
        /// <param name="handler">Handler/delegate that will be invoked when a message is sent<</param>
        /// <param name="parameter">will be passed to the delegate</param>
        public override void SetSentRawMessageHandler(RawMessageHandler handler, object parameter)
        {
            sentMessageHandler = handler;
            sentMessageHandlerParameter = parameter;
        }

        /// <summary>
        /// Determines whether the transmit (send) buffer is full. If true the next send command will throw a ConnectionException
        /// </summary>
        /// <returns><c>true</c> if this instance is send buffer full; otherwise, <c>false</c>.</returns>
        public bool IsTransmitBufferFull()
        {
            if (useSendMessageQueue)
                return false;
            else
                return IsSentBufferFull();
        }

        /// <summary>
        /// Initiates a request to retrieve a file from the remote system.
        /// </summary>
        /// <param name="ca">The communication address of the remote system.</param>
        /// <param name="ioa">The information object address for the file.</param>
        /// <param name="nof">The name of the file to be retrieved.</param>
        /// <param name="receiver">The receiver implementation to handle the file data.</param>
        public override void GetFile(int ca, int ioa, NameOfFile nof, IFileReceiver receiver)
        {
            if (fileClient == null)
                fileClient = new FileClient(this, DebugLog);

            fileClient.RequestFile(ca, ioa, nof, receiver);
        }

        /// <summary>
        /// Initiates a request to retrieve a file from the remote system with a specified timeout.
        /// </summary>
        /// <param name="ca">The communication address of the remote system.</param>
        /// <param name="ioa">The information object address for the file.</param>
        /// <param name="nof">The name of the file to be retrieved.</param>
        /// <param name="receiver">The receiver implementation to handle the file data.</param>
        /// <param name="timeout">The timeout duration in milliseconds for the file request.</param>
        public void GetFile(int ca, int ioa, NameOfFile nof, IFileReceiver receiver, int timeout)
        {
            if (fileClient == null)
                fileClient = new FileClient(this, DebugLog);

            fileClient.Timeout = timeout;

            fileClient.RequestFile(ca, ioa, nof, receiver);
        }

        /// <summary>
        /// Initiates a request to send a file to the remote system.
        /// </summary>
        /// <param name="ca">The communication address of the remote system.</param>
        /// <param name="ioa">The information object address for the file.</param>
        /// <param name="nof">The name of the file to be sent.</param>
        /// <param name="fileProvider">The file provider interface that supplies the file data to be sent.</param>
        public override void SendFile(int ca, int ioa, NameOfFile nof, IFileProvider fileProvider)
        {
            if (fileClient == null)
                fileClient = new FileClient(this, DebugLog);

            fileClient.SendFile(ca, ioa, nof, fileProvider);
        }

        /// <summary>
        /// Requests the directory listing from the remote system.
        /// </summary>
        /// <param name="ca">The communication address of the remote system.</param>
        public void GetDirectory(int ca)
        {
            ASDU getDirectoryAsdu = new ASDU(GetApplicationLayerParameters(), CauseOfTransmission.REQUEST, false, false, 0, ca, false);

            InformationObject io = new FileCallOrSelect(0, NameOfFile.DEFAULT, 0, SelectAndCallQualifier.DEFAULT);

            getDirectoryAsdu.AddInformationObject(io);

            SendASDU(getDirectoryAsdu);
        }
    }
}

