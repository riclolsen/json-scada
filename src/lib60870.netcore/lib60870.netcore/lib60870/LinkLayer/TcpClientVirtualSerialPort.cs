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
using System.IO;
using System.Net.Sockets;
using System.Net;
using System.Threading;

namespace lib60870.linklayer
{

    /// <summary>
    /// TCP client virtual serial port. Can be used to tunnel CS 101 protocol over TCP/IP.
    /// </summary>
    public class TcpClientVirtualSerialPort : Stream
    {
        private int readTimeout = 0;

        private bool debugOutput = false;
        private bool running = false;
        private bool connected = false;

        private string hostname;
        private int tcpPort;

        Socket conSocket = null;
        Stream socketStream = null;
        Thread connectionThread;

        private int connectTimeoutInMs = 1000;
        private int waitRetryConnect = 1000;

        private void DebugLog(string msg)
        {
            if (debugOutput)
            {
                Console.Write("CS101 TCP link layer: ");
                Console.WriteLine(msg);
            }
        }


        /// <summary>
        /// Gets a value indicating whether this <see cref="lib60870.linklayer.TcpClientVirtualSerialPort"/> is connected to a server
        /// </summary>
        /// <value><c>true</c> if connected; otherwise, <c>false</c>.</value>
        public bool Connected
        {
            get
            {
                return this.connected;
            }
        }

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
        /// Initializes a new instance of the <see cref="lib60870.linklayer.TcpClientVirtualSerialPort"/> class.
        /// </summary>
        /// <param name="hostname">IP address of the server</param>
        /// <param name="tcpPort">TCP port of the server</param>
        public TcpClientVirtualSerialPort(String hostname, int tcpPort = 2404)
        {
            this.hostname = hostname;
            this.tcpPort = tcpPort;
        }

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
                throw new SocketException(87); // wrong argument
            }

            if (!running)
                return;

            // Create a TCP/IP  socket.
            conSocket = new Socket(AddressFamily.InterNetwork,
                SocketType.Stream, ProtocolType.Tcp);

            var result = conSocket.BeginConnect(remoteEP, null, null);

            if (!running)
                return;

            bool success = result.AsyncWaitHandle.WaitOne(connectTimeoutInMs, true);
            if (success)
            {
                try
                {
                    conSocket.EndConnect(result);
                    conSocket.NoDelay = true;
                }
                catch (ObjectDisposedException)
                {
                    conSocket = null;

                    DebugLog("ObjectDisposedException -> Connect canceled");

                    throw new SocketException(995); // WSA_OPERATION_ABORTED
                }
            }
            else
            {
                conSocket.Close();
                conSocket = null;

                throw new SocketException(10060); // Connection timed out (WSAETIMEDOUT)
            }
        }

        private void ConnectionThread()
        {
            running = true;

            DebugLog("Starting connection thread");

            while (running)
            {

                try
                {
                    DebugLog("Connecting to " + hostname + ":" + tcpPort);

                    ConnectSocketWithTimeout();

                    socketStream = new NetworkStream(conSocket);

                    connected = true;

                    while (connected)
                    {

                        if (conSocket.Connected == false)
                            break;

                        if (running == false)
                            break;

                        Thread.Sleep(10);
                    }

                    connected = false;

                    if (!this.running)
                        return;

                    if (socketStream != null)
                    {
                        socketStream.Close();
                        conSocket.Dispose();
                        socketStream = null;
                    }


                    if (conSocket != null)
                    {
                        conSocket.Close();
                        conSocket.Dispose();
                        conSocket = null;

                    }

                }
                catch (SocketException e)
                {
                    DebugLog("Failed to connect: " + e.Message);
                    connected = false;
                    socketStream = null;
                    conSocket = null;
                }
					
                if (running)
                    Thread.Sleep(waitRetryConnect);
            }
        }

        /// <summary>
        /// Start the virtual serial port (connect to server)
        /// </summary>
        public void Start()
        {
            if (running == false)
            {
                connectionThread = new Thread(ConnectionThread);

                connectionThread.Start();
            }
        }

        /// <summary>
        /// Stop the virtual serial port
        /// </summary>
        public void Stop()
        {
            if (running == true)
            {
                running = false;
                this.connected = false;

                if (socketStream != null)
                {
                    socketStream.Close();
                    socketStream.Dispose();
                    socketStream = null;
                }

                if (conSocket != null)
                {
				
                    try
                    {
                        conSocket.Shutdown(SocketShutdown.Both);	
                    }
                    catch (SocketException)
                    {
                    }

                    conSocket.Close();
                    conSocket.Dispose();
                    conSocket = null;
                }

                connectionThread.Join();
            }
        }


        /*************************
		 * Stream implementation 
		 */

        public override int Read(byte[] buffer, int offset, int count)
        {
            if (socketStream != null)
            {

                try
                {
                    if (conSocket.Poll(ReadTimeout, SelectMode.SelectRead))
                    {
                        if (connected)
                            return socketStream.Read(buffer, offset, count);
                        else
                            return 0;
                    }
                    else
                        return 0;
                }
                catch (Exception e)
                {
                    DebugLog("Socket error: " + e.ToString());
                    this.connected = false;
                    return 0;
                }

            }
            else
                return 0;
        }

        public override void Write(byte[] buffer, int offset, int count)
        {
            if (socketStream != null)
            {
                try
                {
                    socketStream.Write(buffer, offset, count);
                }
                catch (IOException)
                {
                    connected = false;
                }
            }
        }

        public override bool CanRead
        {
            get
            {
                return true;
            }
        }

        public override bool CanSeek
        {
            get
            {
                return false;
            }
        }

        public override bool CanTimeout
        {
            get
            {
                return true;
            }
        }

        public override bool CanWrite
        {
            get
            {
                return true;
            }
        }

        public override long Length
        {
            get
            {
                throw new NotImplementedException();
            }
        }

        public override long Position
        {
            get
            {
                throw new NotImplementedException();
            }
            set
            {
                throw new NotImplementedException();
            }
        }

        public override int ReadTimeout
        {
            get
            {
                return readTimeout;
            }
            set
            {
                readTimeout = value;
            }
        }

        public override int WriteTimeout
        {
            get
            {
                return base.WriteTimeout;
            }
            set
            {
                base.WriteTimeout = value;
            }
        }

        public override long Seek(long offset, SeekOrigin origin)
        {
            throw new NotImplementedException();
        }

        public override void Flush()
        {
            if (socketStream != null)
                socketStream.Flush();
        }

        public override void SetLength(long value)
        {
            throw new NotImplementedException();
        }
    }
}
