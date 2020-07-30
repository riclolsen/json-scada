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
    /// Connection event handler. Can be used to track connections and accept/deny specific clients.
    /// </summary>
    /// <param name="parameter">User provided paramter</param>
    /// <param name="ipAddress">IP address of the client</param>
    /// <param name="connect">true when client is connecting, false when disconnected</param>
    /// <returns>true when connection is accepted, false otherwise</returns>
    public delegate bool TcpConnectionEventHandler(object parameter,IPAddress ipAddress,bool connect);

    public class TcpServerVirtualSerialPort : Stream
    {
        private int readTimeout = 0;

        private bool debugOutput = false;
        private bool running = false;
        private bool connected = false;

        private string localHostname = "0.0.0.0";
        private int localPort = 2404;
        private Socket listeningSocket;

        Socket conSocket = null;
        Stream socketStream = null;
        Thread acceptThread;

        TcpConnectionEventHandler connectionEventHandler = null;
        object connectionEventHandlerParameter;

        private void DebugLog(string msg)
        {
            if (debugOutput)
            {
                Console.Write("CS101 TCP link layer: ");
                Console.WriteLine(msg);
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

        public TcpServerVirtualSerialPort()
        {
        }

        public void SetConnectionRequestHandler(TcpConnectionEventHandler handler, object parameter)
        {
            this.connectionEventHandler = handler;
            this.connectionEventHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets the local address to used
        /// </summary>
        /// <param name="localAddress">Local address. Use "0.0.0.0" for all interfaces(default)</param>
        public void SetLocalAddress(string localAddress)
        {
            localHostname = localAddress;
        }

        public void SetTcpPort(int tcpPort)
        {
            localPort = tcpPort;
        }

        private void ServerAcceptThread()
        {
            running = true;

            DebugLog("Waiting for connections...");

            while (running)
            {

                try
                {

                    Socket newSocket = listeningSocket.Accept();

                    if (newSocket != null)
                    {
                        DebugLog("New connection");

                        IPEndPoint ipEndPoint = (IPEndPoint)newSocket.RemoteEndPoint;

                        DebugLog("  from IP: " + ipEndPoint.Address.ToString());

                        bool acceptConnection = true;

                        if (connectionEventHandler != null)
                            acceptConnection = connectionEventHandler(connectionEventHandlerParameter, ipEndPoint.Address, true);

                        if (acceptConnection)
                        {

                            conSocket = newSocket;
                            socketStream = new NetworkStream(newSocket);
                            connected = true;

                            while (connected)
                            {

                                if (conSocket.Connected == false)
                                    break;

                                Thread.Sleep(10);
                            }

                            connected = false;

                            if (connectionEventHandler != null)
                                connectionEventHandler(connectionEventHandlerParameter, ipEndPoint.Address, false);

                            socketStream.Close();
                            socketStream = null;
                            conSocket.Close();
                            conSocket = null;

                            DebugLog("Connection from " + ipEndPoint.Address.ToString() + " closed");
                        }
                        else
                            newSocket.Close();
                    }

                }
                catch (Exception)
                {
                    running = false;
                }

            }
        }

        public void Start()
        {
            if (running == false)
            {
                IPAddress ipAddress = IPAddress.Parse(localHostname);
                IPEndPoint localEP = new IPEndPoint(ipAddress, localPort);

                // Create a TCP/IP  socket.
                listeningSocket = new Socket(AddressFamily.InterNetwork, 
                    SocketType.Stream, ProtocolType.Tcp);

                listeningSocket.Bind(localEP);

                listeningSocket.Listen(1);

                acceptThread = new Thread(ServerAcceptThread);

                acceptThread.Start();
            }
        }

        public void Stop()
        {
            if (running == true)
            {
                running = false;
                connected = false;
                listeningSocket.Close();

                acceptThread.Join();
            }
        }


        /*************************
		 * Stream implementation 
		 */

        public override int Read(byte[] buffer, int offset, int count)
        {
            if (socketStream != null)
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

