/*
 *  SerialTransceiverFT12.cs
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
using System.IO;
using System.Threading;

namespace lib60870.linklayer
{
    /// <summary>
    /// Serial transceiver for FT 1.2 type frames
    /// </summary>
    internal class SerialTransceiverFT12
    {

        private Stream serialStream = null;
        private SerialPort port = null;

        private Action<string> DebugLog;

        // link layer paramters - required to determine address (A) field length in FT 1.2 frame
        private LinkLayerParameters linkLayerParameters;

        // timeout used to wait for the message start character
        private int messageTimeout = 50;

        // timeout to wait for next character in a message
        private int characterTimeout = 50;

        private bool fatalError = false;

        public SerialTransceiverFT12(SerialPort port, LinkLayerParameters linkLayerParameters, Action<string> debugLog)
        {
            this.port = port;
            this.serialStream = port.BaseStream;
            this.DebugLog = debugLog;
            this.linkLayerParameters = linkLayerParameters;
        }

        public SerialTransceiverFT12(Stream serialStream, LinkLayerParameters linkLayerParameters, Action<string> debugLog)
        {
            this.port = null;
            this.serialStream = serialStream;
            this.DebugLog = debugLog;
            this.linkLayerParameters = linkLayerParameters;
        }


        public int BaudRate
        {
            get
            {
                if (port != null)
                    return port.BaudRate;
                else
                    return 10000000;
            }
        }

        /// <summary>
        /// Sets the timeouts for receiving messages
        /// </summary>
        /// <param name="messageTimeout">timeout to wait for message start (first byte in the nessage)</param>
        /// <param name="characterTimeout">timeout to wait for next byte (character) in a message</param>
        public void SetTimeouts(int messageTimeout, int characterTimeout)
        {
            this.messageTimeout = messageTimeout;
            this.characterTimeout = characterTimeout;
        }

        /// <summary>
        /// Sends the message over the wire
        /// </summary>
        /// <param name="msg">message data buffer</param>
        /// <param name="msgSize">number of bytes to send</param>
        public void SendMessage(byte[] msg, int msgSize)
        {
            DebugLog("SEND " + BitConverter.ToString(msg, 0, msgSize));

            try
            {
                serialStream.Write(msg, 0, msgSize);
                serialStream.Flush();
            }
            catch(UnauthorizedAccessException)
            {

            }
            
        }

        // read the next block of the message
        private int ReadBytesWithTimeout(byte[] buffer, int startIndex, int count, int timeout)
        {
            int readByte;
            int readBytes = 0;

            try
            {
                serialStream.ReadTimeout = timeout * count;

                while ((readByte = serialStream.ReadByte()) != -1)
                {
                    buffer[startIndex++] = (byte)readByte;

                    readBytes++;

                    if (readBytes >= count)
                        break;
                }
            }
            catch (TimeoutException)
            {
            }
            catch(IOException ex)
            {
                DebugLog("READ: IOException - " + ex.Message);	
            }
            catch (UnauthorizedAccessException)
            {
                if (fatalError == false)
                {
                    if (accessDenied != null)
                        accessDenied(this, EventArgs.Empty);

                    fatalError = true;
                }
            }

            return readBytes;
        }

        private event EventHandler accessDenied = null;

        public void AddPortDeniedHandler (EventHandler eventHandler)
        {
            accessDenied += eventHandler;
        }

        public void ReadNextMessage(byte[] buffer, Action<byte[], int> messageHandler)
        {
            // NOTE: there is some basic decoding required to determine message start/end
            //       and synchronization failures.

            try
            {

                int read = ReadBytesWithTimeout(buffer, 0, 1, messageTimeout);

                if (read == 1)
                {

                    if (buffer[0] == 0x68)
                    {

                        int bytesRead = ReadBytesWithTimeout(buffer, 1, 1, characterTimeout);

                        if (bytesRead == 1)
                        {

                            int msgSize = buffer[1];

                            msgSize += 4;

                            int readBytes = ReadBytesWithTimeout(buffer, 2, msgSize, characterTimeout);

                            if (readBytes == msgSize)
                            {

                                msgSize += 2;

                                messageHandler(buffer, msgSize);
                            }
                            else
                                DebugLog("RECV: Timeout reading variable length frame msgSize = " + msgSize + " readBytes = " +
                                    readBytes);
                        }
                        else
                        {
                            DebugLog("RECV: SYNC ERROR 1!");
                        }
                    }
                    else if (buffer[0] == 0x10)
                    {

                        int msgSize = 3 + linkLayerParameters.AddressLength;

                        int readBytes = ReadBytesWithTimeout(buffer, 1, msgSize, characterTimeout);

                        if (readBytes == msgSize)
                        {

                            msgSize += 1;

                            messageHandler(buffer, msgSize);
                        }
                        else
                            DebugLog("RECV: Timeout reading fixed length frame msgSize = " + msgSize + " readBytes = " +
                                readBytes);
                    }
                    else if (buffer[0] == 0xe5)
                    {
                        int msgSize = 1;

                        messageHandler(buffer, msgSize);
                    }
                    else
                    {
                        DebugLog("RECV: SYNC ERROR 2! value = " + buffer[0]);
                    }
                }

            }
            catch (TimeoutException)
            {
            }
        }
    }
}

