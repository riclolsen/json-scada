/*
 *  Copyright 2017-2019 MZ Automation GmbH
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

namespace lib60870.CS101
{
    [Serializable]
    public class ASDUQueueException : Exception
    {
        public ASDUQueueException()
        {
        }


        public ASDUQueueException(string message)
            : base(message)
        {
        }

        public ASDUQueueException(string message, Exception innerException)
            : base(message, innerException)
        {
            
        }
    }

    /// <summary>
    /// Provides functions to be used in Slave callbacks to send data back to the master
    /// </summary>
    public interface IMasterConnection
    {
        void SendASDU(ASDU asdu);

        void SendACT_CON(ASDU asdu, bool negative);

        void SendACT_TERM(ASDU asdu);

        ApplicationLayerParameters GetApplicationLayerParameters();
    }

    /// <summary>
    /// Handler for interrogation command (C_IC_NA_1 - 100).
    /// </summary>
	public delegate bool InterrogationHandler(object parameter,IMasterConnection connection,ASDU asdu,byte qoi);

    /// <summary>
    /// Handler for counter interrogation command (C_CI_NA_1 - 101).
    /// </summary>
	public delegate bool CounterInterrogationHandler(object parameter,IMasterConnection connection,ASDU asdu,byte qoi);

    /// <summary>
    /// Handler for read command (C_RD_NA_1 - 102)
    /// </summary>
	public delegate bool ReadHandler(object parameter,IMasterConnection connection,ASDU asdu,int ioa);

    /// <summary>
    /// Handler for clock synchronization command (C_CS_NA_1 - 103)
    /// </summary>
	public delegate bool ClockSynchronizationHandler(object parameter,IMasterConnection connection,ASDU asdu,CP56Time2a newTime);

    /// <summary>
    /// Handler for reset process command (C_RP_NA_1 - 105)
    /// </summary>
	public delegate bool ResetProcessHandler(object parameter,IMasterConnection connection,ASDU asdu,byte  qrp);

    /// <summary>
    /// Handler for delay acquisition command (C_CD_NA:1 - 106)
    /// </summary>
	public delegate bool DelayAcquisitionHandler(object parameter,IMasterConnection connection,ASDU asdu,CP16Time2a delayTime);


    /// <summary>
    /// Handler for ASDUs that are not handled by other handlers (default handler)
    /// </summary>
	public delegate bool ASDUHandler(object parameter,IMasterConnection connection,ASDU asdu);

    public class Slave
    {

        protected bool debugOutput;

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

        public InterrogationHandler interrogationHandler = null;
        public object InterrogationHandlerParameter = null;

        public CounterInterrogationHandler counterInterrogationHandler = null;
        public object counterInterrogationHandlerParameter = null;

        public ReadHandler readHandler = null;
        public object readHandlerParameter = null;

        public ClockSynchronizationHandler clockSynchronizationHandler = null;
        public object clockSynchronizationHandlerParameter = null;

        public ResetProcessHandler resetProcessHandler = null;
        public object resetProcessHandlerParameter = null;

        public DelayAcquisitionHandler delayAcquisitionHandler = null;
        public object delayAcquisitionHandlerParameter = null;

        public  ASDUHandler asduHandler = null;
        public object asduHandlerParameter = null;

        internal FileReadyHandler fileReadyHandler = null;
        internal object fileReadyHandlerParameter = null;

        /// <summary>
        /// Sets a callback for interrogaton requests.
        /// </summary>
        /// <param name="handler">The interrogation request handler callback function</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetInterrogationHandler(InterrogationHandler handler, object parameter)
        {
            this.interrogationHandler = handler;
            this.InterrogationHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets a callback for counter interrogaton requests.
        /// </summary>
        /// <param name="handler">The counter interrogation request handler callback function</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetCounterInterrogationHandler(CounterInterrogationHandler handler, object parameter)
        {
            this.counterInterrogationHandler = handler;
            this.counterInterrogationHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets a callback for read requests.
        /// </summary>
        /// <param name="handler">The read request handler callback function</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetReadHandler(ReadHandler handler, object parameter)
        {
            this.readHandler = handler;
            this.readHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets a callback for the clock synchronization request.
        /// </summary>
        /// <param name="handler">The clock synchronization request handler callback function</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetClockSynchronizationHandler(ClockSynchronizationHandler handler, object parameter)
        {
            this.clockSynchronizationHandler = handler;
            this.clockSynchronizationHandlerParameter = parameter;
        }

        public void SetResetProcessHandler(ResetProcessHandler handler, object parameter)
        {
            this.resetProcessHandler = handler;
            this.resetProcessHandlerParameter = parameter;
        }

        public void SetDelayAcquisitionHandler(DelayAcquisitionHandler handler, object parameter)
        {
            this.delayAcquisitionHandler = handler;
            this.delayAcquisitionHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets a callback to handle ASDUs (commands, requests) form clients. This callback can be used when
        /// no other callback handles the message from the client/master.
        /// </summary>
        /// <param name="handler">The ASDU callback function</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetASDUHandler(ASDUHandler handler, object parameter)
        {
            this.asduHandler = handler;
            this.asduHandlerParameter = parameter;
        }

        /// <summary>
        /// Sets a callback handler that is called when a file ready message is received from a master
        /// </summary>
        /// <param name="handler">callback function to be called</param>
        /// <param name="parameter">user provided parameter that is passed to the callback</param>
        public void SetFileReadyHandler(FileReadyHandler handler, object parameter)
        {
            fileReadyHandler = handler;
            fileReadyHandlerParameter = parameter;
        }

        protected FilesAvailable filesAvailable = new FilesAvailable();

        /// <summary>
        /// Gets the available files that are registered with the file server
        /// </summary>
        /// <returns>The available files.</returns>
        public FilesAvailable GetAvailableFiles()
        {
            return filesAvailable;
        }

        /// <summary>
        /// Gets or sets the file service timeout.
        /// </summary>
        /// <value>The file service timeout in ms</value>
        public virtual int FileTimeout { get; set; }
    }

}

