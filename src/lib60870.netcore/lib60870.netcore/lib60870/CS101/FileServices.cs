/*
 *  FileServices.cs
 *
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
using System.Collections.Generic;

namespace lib60870.CS101
{

    public enum FileErrorCode
    {
        SUCCESS,
        TIMEOUT,
        FILE_NOT_READY,
        SECTION_NOT_READY,
        UNKNOWN_CA,
        UNKNOWN_IOA,
        UNKNOWN_SERVICE,
        PROTOCOL_ERROR,
        ABORTED_BY_REMOTE
    }

    public interface IFileReceiver
    {
        void Finished(FileErrorCode result);

        void SegmentReceived(byte sectionName, int offset, int size, byte[] data);
    }

    public interface IFileProvider
    {

        /// <summary>
        /// Returns the CA (Comman address) of the file
        /// </summary>
        /// <returns>The CA</returns>
        int GetCA();

        /// <summary>
        /// Returns the IOA (information object address of the file)
        /// </summary>
        /// <returns>The IOA</returns>
        int GetIOA();

        /// <summary>
        /// Gets the type ("name") of the file
        /// </summary>
        /// <returns>The file type</returns>
        NameOfFile GetNameOfFile();

        DateTime GetFileDate();

        /// <summary>
        /// Gets the size of the file in bytes
        /// </summary>
        /// <returns>The file size in bytes</returns>
        int GetFileSize();

        /// <summary>
        /// Gets the size of a section in byzes
        /// </summary>
        /// <returns>The section size in bytes or -1 if the section does not exist</returns>
        /// <param name="sectionNumber">Number of section (starting with 0)</param>
        int GetSectionSize(int sectionNumber);

        /// <summary>
        /// Gets the segment data.
        /// </summary>
        /// <returns><c>true</c>, if segment data was gotten, <c>false</c> otherwise.</returns>
        /// <param name="sectionNumber">Section number.</param>
        /// <param name="offset">Offset.</param>
        /// <param name="segmentSize">Segment size.</param>
        /// <param name="segmentData">Segment data.</param>
        bool GetSegmentData(int sectionNumber, int offset, int segmentSize, byte[] segmentData);

        /// <summary>
        /// Indicates that the transfer is complete. When success equals true the file data can be deleted
        /// </summary>
        /// <param name="success">If set to <c>true</c> success.</param>
        void TransferComplete(bool success);
    }

    /// <summary>
    /// File ready handler. Will be called by the FileServer when a master sends a FILE READY (file download announcement) message to the slave.
    /// </summary>
	public delegate IFileReceiver FileReadyHandler(object parameter,int ca,int ioa,NameOfFile nof,int lengthOfFile);

    /// <summary>
    /// Simple implementation of IFileProvider that can be used to provide transparent files. Derived classed should override the
    /// TransferComplete method.
    /// </summary>
    public class TransparentFile : IFileProvider
    {
        private List<byte[]> sections = new List<byte[]>();

        private DateTime time = DateTime.MinValue;

        private int ca;
        private int ioa;
        private NameOfFile nof;

        public TransparentFile(int ca, int ioa, NameOfFile nof)
        {
            this.ca = ca;
            this.ioa = ioa;
            this.nof = nof;
            time = DateTime.Now;
        }

        public void AddSection(byte[] section)
        {
            sections.Add(section);
        }

        public int GetCA()
        {
            return ca;
        }

        public int GetIOA()
        {
            return ioa;
        }

        public NameOfFile GetNameOfFile()
        {
            return nof;
        }

        public DateTime GetFileDate()
        {
            return time;
        }

        public int GetFileSize()
        {
            int fileSize = 0;

            foreach (byte[] section in sections)
                fileSize += section.Length;

            return fileSize;
        }

        public int GetSectionSize(int sectionNumber)
        {
            if (sectionNumber < sections.Count)
                return sections[sectionNumber].Length;
            else
                return -1;
        }

        public bool GetSegmentData(int sectionNumber, int offset, int segmentSize, byte[] segmentData)
        {
            if ((sectionNumber >= sections.Count) || (sectionNumber < 0))
                return false;

            byte[] section = sections[sectionNumber];

            if (offset + segmentSize > section.Length)
                return false;

            for (int i = 0; i < segmentSize; i++)
                segmentData[i] = section[i + offset];

            return true;
        }

        public virtual void TransferComplete(bool success)
        {
        }
    }


    internal enum FileClientState
    {
        IDLE,

        /* states for file upload (monitor direction) */
            
        WAITING_FOR_FILE_READY,

        WAITING_FOR_SECTION_READY, /* or for LAST_SECTION */

        RECEIVING_SECTION, /* waiting for SEGMENT or LAST SEGMENT */

        /* states for file download (control direction) */

        WAITING_FOR_REQUEST_FILE,

        SECTION_READY,

        SEND_SECTION,

        WAITING_FOR_SECTION_ACK,

        WAITING_FOR_FILE_ACK

    }

    delegate void DebugLogger(string message);

    internal class FileClient
    {
        private FileClientState state = FileClientState.IDLE;
        private Master master;

        private int ca;
        private int ioa;
        private int numberOfSection;
        private NameOfFile nof;
        private IFileReceiver fileReceiver = null;
        private IFileProvider fileProvider = null;

        private DebugLogger DebugLog;

        private int maxSegmentSize = 0;

        private int currentSectionSize = 0;
        private int currentSectionOffset = 0;
        private byte sectionChecksum = 0;
        private byte fileChecksum = 0;

        private long timeout = 3000;
        private long lastSentTime = 0;

        public FileClient(Master master, DebugLogger debugLog)
        {
            this.master = master;
            DebugLog = debugLog;
            maxSegmentSize = FileSegment.GetMaxDataSize (master.GetApplicationLayerParameters());
        }

        /// <summary>
        /// Gets or sets the timeout for file transfers
        /// </summary>
        /// <value>timeout in ms</value>
        public long Timeout {
            get {
                return timeout;
            }
            set {
                timeout = value;
            }
        }

        private ASDU NewAsdu(InformationObject io)
        {
            ASDU asdu = new ASDU(master.GetApplicationLayerParameters(), CauseOfTransmission.FILE_TRANSFER, false, false, 0, ca, false);

            asdu.AddInformationObject(io);

            return asdu;
        }

        private void SendLastSegment ()
        {
            ASDU fileAsdu = NewAsdu(new FileLastSegmentOrSection (ioa, nof, (byte)numberOfSection,
                    LastSectionOrSegmentQualifier.SECTION_TRANSFER_WITHOUT_DEACT,
                    sectionChecksum));

            fileChecksum += sectionChecksum;
            sectionChecksum = 0;

            DebugLog ("Send LAST SEGMENT (NoS=" + numberOfSection + ")");

            master.SendASDU (fileAsdu);
        }

        private byte CalculateChecksum (byte [] data)
        {
            byte checksum = 0;

            foreach (byte octet in data) {
                checksum += octet;
            }

            return checksum;
        }

        private bool SendSegment ()
        {
            int currentSegmentSize = currentSectionSize - currentSectionOffset;

            if (currentSegmentSize > 0) {
                if (currentSegmentSize > maxSegmentSize)
                    currentSegmentSize = maxSegmentSize;

                byte [] segmentData = new byte [currentSegmentSize];

                fileProvider.GetSegmentData (numberOfSection - 1,
                    currentSectionOffset,
                    currentSegmentSize,
                    segmentData);

                ASDU fileAsdu = NewAsdu (new FileSegment (ioa, nof, (byte)numberOfSection, segmentData));

                lastSentTime = SystemUtils.currentTimeMillis ();

                master.SendASDU (fileAsdu);

                sectionChecksum += CalculateChecksum (segmentData); 

                DebugLog ("Send SEGMENT (NoS=" + numberOfSection + ", CHS=" + sectionChecksum + ")");
                currentSectionOffset += currentSegmentSize;

                return true;
            } else
                return false;

        }

        private void ResetStateToIdle()
        {
            fileReceiver = null;
            fileProvider = null;
            fileChecksum = 0;
            state = FileClientState.IDLE;
        }

        private void AbortFileTransfer(FileErrorCode errorCode)
        {
            ASDU deactivateFile = NewAsdu(new FileCallOrSelect(ioa, nof, 0, SelectAndCallQualifier.DEACTIVATE_FILE));

            master.SendASDU(deactivateFile);

            lastSentTime = SystemUtils.currentTimeMillis ();

            if (fileReceiver != null)
                fileReceiver.Finished(errorCode);

            ResetStateToIdle();
        }

        private void FileUploadFailed ()
        {
            if (fileProvider != null)
                fileProvider.TransferComplete (false);
            ResetStateToIdle ();
        }

        public bool HandleFileAsdu(ASDU asdu)
        {
            bool asduHandled = true;

            switch (asdu.TypeId)
            {

            case TypeID.F_SC_NA_1: /* File/Section/Directory Call/Select */

                DebugLog ("Received F_SC_NA_1 (select/call)");

                if (state == FileClientState.WAITING_FOR_FILE_READY) /* file download */
                {
                    FileErrorCode errCode = FileErrorCode.PROTOCOL_ERROR;

                    if (asdu.Cot == CauseOfTransmission.UNKNOWN_TYPE_ID)
                        errCode = FileErrorCode.UNKNOWN_SERVICE;
                    else if (asdu.Cot == CauseOfTransmission.UNKNOWN_COMMON_ADDRESS_OF_ASDU)
                        errCode = FileErrorCode.UNKNOWN_CA;
                    else if (asdu.Cot == CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS)
                        errCode = FileErrorCode.UNKNOWN_IOA;

                    if (fileReceiver != null)
                        fileReceiver.Finished (errCode);

                    ResetStateToIdle ();
                } else if (state == FileClientState.WAITING_FOR_REQUEST_FILE) /* file upload */
                  {
                    if ((asdu.Ca == ca)) {

                        numberOfSection = 1;
                        currentSectionSize = fileProvider.GetSectionSize (0);

                        ASDU sectionReady = NewAsdu (new SectionReady (ioa, nof, 1, currentSectionSize, false));
                        master.SendASDU (sectionReady);

                        lastSentTime = SystemUtils.currentTimeMillis ();

                        state = FileClientState.SECTION_READY;
                    } else {
                        fileProvider.TransferComplete (false);
                        ResetStateToIdle ();
                    }
                } else if (state == FileClientState.SECTION_READY) {

                    if ((asdu.Ca == ca)) {

                        // send first segment

                        currentSectionOffset = 0;

                        SendSegment ();

                        state = FileClientState.SEND_SECTION;
                    }

                } else {
                    if (fileReceiver != null)
                        fileReceiver.Finished (FileErrorCode.PROTOCOL_ERROR);

                    ResetStateToIdle ();
                }

                break;

            case TypeID.F_FR_NA_1: /* File ready */

                DebugLog("Received F_FR_NA_1 (file ready)");

                if (state == FileClientState.WAITING_FOR_FILE_READY) {

                    FileReady fileReady = (FileReady)asdu.GetElement (0);

                    if ((asdu.Ca == ca) && (fileReady.ObjectAddress == ioa) && (fileReady.NOF == nof)) {

                        if (fileReady.Positive) {

                            /* send call file */

                            ASDU callFile = NewAsdu (new FileCallOrSelect (ioa, nof, 0, SelectAndCallQualifier.REQUEST_FILE));
                            master.SendASDU (callFile);

                            lastSentTime = SystemUtils.currentTimeMillis ();

                            DebugLog ("Send CALL FILE");

                            state = FileClientState.WAITING_FOR_SECTION_READY;

                        } else {
                            if (fileReceiver != null)
                                fileReceiver.Finished (FileErrorCode.FILE_NOT_READY);

                            ResetStateToIdle ();
                        }

                    } else {
                        DebugLog ("Unexpected CA, IOA, or NOF");

                        if (fileReceiver != null)
                            fileReceiver.Finished (FileErrorCode.PROTOCOL_ERROR);

                        ResetStateToIdle ();
                    }


                } else if (state == FileClientState.IDLE) {
                
                    state = FileClientState.WAITING_FOR_SECTION_READY;

                } else if (state == FileClientState.WAITING_FOR_REQUEST_FILE) {

                    if (asdu.IsNegative) {
                        DebugLog ("Slave rejected file download: " + asdu.Cot.ToString ());
                    } else {
                        DebugLog ("Unexpected file ready while trying to start file download");
                    }

                    if (fileProvider != null)
                        fileProvider.TransferComplete (false);

                } else {
                    AbortFileTransfer (FileErrorCode.PROTOCOL_ERROR);
                }

                break;

            case TypeID.F_SR_NA_1: /* Section ready */

                DebugLog ("Received F_SR_NA_1 (section ready)");

                if (state == FileClientState.WAITING_FOR_SECTION_READY) {

                    SectionReady sc = (SectionReady)asdu.GetElement (0);

                    if (sc.NotReady == false) {
                        DebugLog ("Received SECTION READY(NoF=" + sc.NOF + ", NoS=" + sc.NameOfSection + ")");

                        ASDU callSection = NewAsdu (new FileCallOrSelect (ioa, nof, sc.NameOfSection, SelectAndCallQualifier.REQUEST_SECTION));
                        master.SendASDU (callSection);

                        lastSentTime = SystemUtils.currentTimeMillis ();

                        DebugLog ("Send CALL SECTION(NoF=" + sc.NOF + ", NoS=" + sc.NameOfSection + ")");

                        currentSectionOffset = 0;
                        sectionChecksum = 0;
                        state = FileClientState.RECEIVING_SECTION;

                    } else {
                        AbortFileTransfer (FileErrorCode.SECTION_NOT_READY);
                    }

                } else if (state == FileClientState.IDLE) {
                } else {
                    if (fileReceiver != null)
                        fileReceiver.Finished (FileErrorCode.PROTOCOL_ERROR);

                    ResetStateToIdle ();
                }

                break;

            case TypeID.F_SG_NA_1: /* Segment */

                DebugLog ("Received F_SG_NA_1 (segment)");

                if (state == FileClientState.RECEIVING_SECTION) {

                    FileSegment segment = (FileSegment)asdu.GetElement (0);

                    DebugLog ("Received segment (NoS=" + segment.NameOfSection + ", LoS=" + segment.LengthOfSegment + ")");

                    sectionChecksum += CalculateChecksum (segment.SegmentData);

                    if (fileReceiver != null) {
                        fileReceiver.SegmentReceived (segment.NameOfSection, currentSectionOffset, segment.LengthOfSegment, segment.SegmentData);
                    }

                    currentSectionOffset += segment.LengthOfSegment;

                } else if (state == FileClientState.IDLE) {
                } else {
                    AbortFileTransfer (FileErrorCode.PROTOCOL_ERROR);
                }

                break;


            case TypeID.F_LS_NA_1: /* Last segment or section */

                DebugLog ("Received F_LS_NA_1 (last segment/section)");

                if (state != FileClientState.IDLE) {

                    FileLastSegmentOrSection lastSection = (FileLastSegmentOrSection)asdu.GetElement (0);

                    if (lastSection.LSQ == LastSectionOrSegmentQualifier.SECTION_TRANSFER_WITHOUT_DEACT) {

                        if (state == FileClientState.RECEIVING_SECTION) {

                            ASDU segmentAck;

                            if (lastSection.CHS == sectionChecksum) {
                                segmentAck = NewAsdu (new FileACK (ioa, nof, lastSection.NameOfSection, AcknowledgeQualifier.POS_ACK_SECTION, FileError.DEFAULT));
                                DebugLog ("Send SEGMENT ACK");
                            }
                            else
                            {
                                segmentAck = NewAsdu (new FileACK (ioa, nof, lastSection.NameOfSection, AcknowledgeQualifier.NEG_ACK_SECTION, FileError.CHECKSUM_FAILED));
                                DebugLog ("checksum check failed! Send SEGMENT NACK");
                            }

                            master.SendASDU (segmentAck);

                            lastSentTime = SystemUtils.currentTimeMillis ();

                            state = FileClientState.WAITING_FOR_SECTION_READY;
                        } else {
                            AbortFileTransfer (FileErrorCode.PROTOCOL_ERROR);
                        }
                    } else if (lastSection.LSQ == LastSectionOrSegmentQualifier.FILE_TRANSFER_WITH_DEACT) {
                        /* slave aborted transfer */

                        if (fileReceiver != null)
                            fileReceiver.Finished (FileErrorCode.ABORTED_BY_REMOTE);

                        ResetStateToIdle ();
                    } else if (lastSection.LSQ == LastSectionOrSegmentQualifier.FILE_TRANSFER_WITHOUT_DEACT) {

                        if (state == FileClientState.WAITING_FOR_SECTION_READY) {
                            ASDU fileAck = NewAsdu (new FileACK (ioa, nof, lastSection.NameOfSection, AcknowledgeQualifier.POS_ACK_FILE, FileError.DEFAULT));

                            master.SendASDU (fileAck);

                            lastSentTime = SystemUtils.currentTimeMillis ();

                            DebugLog ("Send FILE ACK");

                            if (fileReceiver != null)
                                fileReceiver.Finished (FileErrorCode.SUCCESS);

                            ResetStateToIdle ();
                        } else {

                            DebugLog ("Illegal state: " + state.ToString ());

                            AbortFileTransfer (FileErrorCode.PROTOCOL_ERROR);
                        }
                    }
                }

                break;

            case TypeID.F_AF_NA_1: /* Section or File ACK */

                DebugLog ("Received F_AF_NA_1 (section/file ACK)");

                FileACK ack = (FileACK)asdu.GetElement (0);

                if (state == FileClientState.WAITING_FOR_SECTION_ACK) {
                    if ((asdu.Ca == ca) && (asdu.Cot == CauseOfTransmission.FILE_TRANSFER)) {

                        if (ack.AckQualifier == AcknowledgeQualifier.POS_ACK_SECTION) {

                            numberOfSection++;

                            int nextSectionSize = fileProvider.GetSectionSize (numberOfSection - 1);

                            if (nextSectionSize > 0) {
                                currentSectionSize = nextSectionSize;
                                currentSectionOffset = 0;

                                ASDU sectionReady = NewAsdu (new SectionReady (ioa, nof, (byte)numberOfSection, currentSectionSize, false));
                                master.SendASDU (sectionReady);

                                lastSentTime = SystemUtils.currentTimeMillis ();

                                state = FileClientState.SECTION_READY;
                            } else {

                                ASDU lastSection = NewAsdu (new FileLastSegmentOrSection (ioa, nof, (byte)numberOfSection, LastSectionOrSegmentQualifier.FILE_TRANSFER_WITHOUT_DEACT, fileChecksum));
                                master.SendASDU (lastSection);

                                lastSentTime = SystemUtils.currentTimeMillis ();

                                state = FileClientState.WAITING_FOR_FILE_ACK;
                            }

                        } else {
                            FileUploadFailed ();
                        }

                    } else {
                        FileUploadFailed ();
                    }
                } else if (state == FileClientState.WAITING_FOR_FILE_ACK) {
                    if ((asdu.Ca == ca) && (asdu.Cot == CauseOfTransmission.FILE_TRANSFER)) {

                        if (ack.AckQualifier == AcknowledgeQualifier.POS_ACK_FILE) {
                            if (fileProvider != null)
                                fileProvider.TransferComplete (true);

                            ResetStateToIdle ();

                        } else {
                            FileUploadFailed ();
                        }

                    } else {
                        FileUploadFailed ();
                    }
                }

                break;

            default:

                asduHandled = false;
                break;
            }


            return asduHandled;
        }

        public void HandleFileService()
        {

            if (state == FileClientState.SEND_SECTION) 
            {
                if (SendSegment () == false) {
                    SendLastSegment ();
                    state = FileClientState.WAITING_FOR_SECTION_ACK;
                }
            }

            // Check for timeout
            if (state != FileClientState.IDLE) 
            {
                if (SystemUtils.currentTimeMillis () > lastSentTime + timeout)
                {
                    DebugLog ("Abort file transfer due to timeout");

                    if (fileProvider != null)
                        fileProvider.TransferComplete (false);

                    if (fileReceiver != null)
                        fileReceiver.Finished (FileErrorCode.TIMEOUT);

                    ResetStateToIdle ();
                }
            }

        }

        public void RequestFile(int ca, int ioa, NameOfFile nof, IFileReceiver fileReceiver)
        {
            this.ca = ca;
            this.ioa = ioa;
            this.nof = nof;
            this.fileReceiver = fileReceiver;

            ASDU selectFile = NewAsdu(new FileCallOrSelect(ioa, nof, 0, SelectAndCallQualifier.SELECT_FILE));

            master.SendASDU(selectFile);

            lastSentTime = SystemUtils.currentTimeMillis ();

            state = FileClientState.WAITING_FOR_FILE_READY;
        }

        public void SendFile (int ca, int ioa, NameOfFile nof, IFileProvider fileProvider)
        {
            this.ca = ca;
            this.ioa = ioa;
            this.nof = nof;
            this.fileProvider = fileProvider;

            ASDU fileReady = NewAsdu (new FileReady (ioa, nof, fileProvider.GetFileSize (), true));

            master.SendASDU (fileReady);

            lastSentTime = SystemUtils.currentTimeMillis ();

            state = FileClientState.WAITING_FOR_REQUEST_FILE;
        }
    }

    internal enum FileServerState
    {
        UNSELECTED_IDLE,
        WAITING_FOR_FILE_CALL,
        WAITING_FOR_SECTION_CALL,
        TRANSMIT_SECTION,
        WAITING_FOR_SECTION_ACK,
        WAITING_FOR_FILE_ACK,
        SEND_ABORT,
        TRANSFER_COMPLETED,

        WAITING_FOR_SECTION_READY,
        RECEIVE_SECTION,
    }

    /// <summary>
    /// Encapsulates a IFileProvider object to add some state information
    /// </summary>
    internal class CS101n104File
    {

        public CS101n104File(IFileProvider file)
        {
            this.provider = file;
        }

        public IFileProvider provider = null;
        public object selectedBy = null;

    }

    /// <summary>
    /// Represents the available files in a file client or file server
    /// </summary>
    public class FilesAvailable
    {

        private List<CS101n104File> availableFiles = new List<CS101n104File>();

        internal CS101n104File GetFile(int ca, int ioa, NameOfFile nof)
        {
            lock (availableFiles)
            {

                foreach (CS101n104File file in availableFiles)
                {
                    if ((file.provider.GetCA() == ca) && (file.provider.GetIOA() == ioa))
                    {

                        if (nof == NameOfFile.DEFAULT)
                            return file;
                        else
                        {

                            if (nof == file.provider.GetNameOfFile())
                                return file;
                        }
                    }
                }
            }

            return null;
        }

        internal void SendDirectoy(IMasterConnection masterConnection, bool spontaneous)
        {
            CauseOfTransmission cot;

            if (spontaneous)
                cot = CauseOfTransmission.SPONTANEOUS;
            else
                cot = CauseOfTransmission.REQUEST;

            lock (availableFiles)
            {

                int size = availableFiles.Count;
                int i = 0;

                int currentCa = -1;
                int currentIOA = -1;

                ASDU directoryAsdu = null; 

                foreach (CS101n104File file in availableFiles)
                {
				
                    bool newAsdu = false;

                    if (file.provider.GetCA() != currentCa)
                    {
                        currentCa = file.provider.GetCA();
                        newAsdu = true;
                    }

                    if (currentIOA != (file.provider.GetIOA() - 1))
                    {
                        newAsdu = true;
                    }

                    if (newAsdu)
                    {
                        if (directoryAsdu != null)
                        {
                            masterConnection.SendASDU(directoryAsdu);
                            directoryAsdu = null;
                        }
                    }

                    currentIOA = file.provider.GetIOA();

                    i++;

                    if (directoryAsdu == null)
                    {
                        directoryAsdu = new ASDU(masterConnection.GetApplicationLayerParameters(), cot, false, false, 0, currentCa, true);
                    }

                    bool lastFile = (i == size);

                    byte sof = 0;

                    if (lastFile)
                        sof = 0x20;

                    InformationObject io = new FileDirectory(currentIOA, file.provider.GetNameOfFile(), file.provider.GetFileSize(), sof, new CP56Time2a(file.provider.GetFileDate()));

                    if (directoryAsdu.AddInformationObject(io) == false)
                    {
                        masterConnection.SendASDU(directoryAsdu);

                        directoryAsdu = new ASDU(masterConnection.GetApplicationLayerParameters(), cot, false, false, 0, currentCa, true);
                        directoryAsdu.AddInformationObject(io);
                    }
                }

                if (directoryAsdu != null)
                {
                    masterConnection.SendASDU(directoryAsdu);
                }

            }
        }

        /// <summary>
        /// Adds a file to the list of available files
        /// </summary>
        /// <param name="file">file to add</param>
        public void AddFile(IFileProvider file)
        {
            lock (availableFiles)
            {

                availableFiles.Add(new CS101n104File(file));
            }
				
        }

        /// <summary>
        /// Removes a file from the list of available files
        /// </summary>
        /// <param name="file">file to remove</param>
        public void RemoveFile(IFileProvider file)
        {
            lock (availableFiles)
            {

                foreach (CS101n104File availableFile in availableFiles)
                {

                    if (availableFile.provider == file)
                    {
                        availableFiles.Remove(availableFile);
                        return;
                    }

                }
            }
        }

        /// <summary>
        /// Gets the list of available files
        /// </summary>
        /// <returns>the list of available files</returns>
        public List<IFileProvider> GetFiles ()
        {
            List<IFileProvider> files = new List<IFileProvider> ();

            foreach (CS101n104File file in availableFiles) {
                files.Add (file.provider);
            }

            return files;
        }

    }



    internal class FileServer
    {

        public FileServer(IMasterConnection masterConnection, FilesAvailable availableFiles, DebugLogger logger)
        {
            transferState = FileServerState.UNSELECTED_IDLE;
            alParameters = masterConnection.GetApplicationLayerParameters();
            maxSegmentSize = FileSegment.GetMaxDataSize(alParameters);
            this.availableFiles = availableFiles;
            this.logger = logger;
            this.connection = masterConnection;
        }

        private FilesAvailable availableFiles;

        private CS101n104File selectedFile;

        private DebugLogger logger;

        private ApplicationLayerParameters alParameters;

        private IMasterConnection connection;
        private int maxSegmentSize;

        private byte currentSectionNumber;
        private int currentSectionSize;
        private int currentSectionOffset;
        private byte sectionChecksum = 0;
        private byte fileChecksum = 0;

        private long timeout = 3000;
        private long lastSentTime = 0;

        private int ca;
        private int ioa;
        private NameOfFile nof;


        private FileServerState transferState;

        private FileReadyHandler fileReadyHandler = null;
        private object fileReadyHandlerParameter = null;

        private IFileReceiver fileReceiver = null;

        public void SetFileReadyHandler(FileReadyHandler handler, object parameter)
        {
            fileReadyHandler = handler;
            fileReadyHandlerParameter = parameter;
        }

        /// <summary>
        /// Gets or sets the timeout for file transfers
        /// </summary>
        /// <value>timeout in ms</value>
        public long Timeout 
        {
            get 
            {
                return timeout;
            }
            set 
            {
                timeout = value;
            }
        }

        public bool HandleFileAsdu(ASDU asdu)
        {
            bool handled = true;

            switch (asdu.TypeId)
            {
            case TypeID.F_FR_NA_1: /* File Ready */

                logger ("Received file ready F_FR_NA_1");

                if (fileReadyHandler != null) {

                    FileReady fileReady = (FileReady)asdu.GetElement (0);

                    fileReceiver = fileReadyHandler (fileReadyHandlerParameter, asdu.Ca, fileReady.ObjectAddress, fileReady.NOF, fileReady.LengthOfFile);

                    if (fileReceiver == null) {
                        asdu.IsNegative = true;
                        asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                        connection.SendASDU (asdu);
                    } else {

                        ca = asdu.Ca;
                        ioa = fileReady.ObjectAddress;
                        nof = fileReady.NOF;

                        // send call file

                        ASDU callFile = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                        callFile.AddInformationObject (new FileCallOrSelect (fileReady.ObjectAddress, fileReady.NOF, 0, SelectAndCallQualifier.REQUEST_FILE));

                        connection.SendASDU (callFile);

                        lastSentTime = SystemUtils.currentTimeMillis ();
                        transferState = FileServerState.WAITING_FOR_SECTION_READY;
                    }

                } else {
                    asdu.IsNegative = true;
                    asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                    connection.SendASDU (asdu);
                }

                break;

            case TypeID.F_SR_NA_1: /* Section Ready */

                if (transferState == FileServerState.WAITING_FOR_SECTION_READY) {

                    SectionReady sectionReady = (SectionReady)asdu.GetElement (0);

                    currentSectionNumber = sectionReady.NameOfSection;
                    currentSectionOffset = 0;
                    currentSectionSize = sectionReady.LengthOfSection;

                    // send call section

                    ASDU callSection = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, ca, false);

                    callSection.AddInformationObject (new FileCallOrSelect (ioa, nof, (byte)currentSectionNumber, SelectAndCallQualifier.REQUEST_SECTION));

                    connection.SendASDU (callSection);
                    lastSentTime = SystemUtils.currentTimeMillis ();

                    transferState = FileServerState.RECEIVE_SECTION;
                } else {

                }

                break;

            case TypeID.F_SG_NA_1: /* Segment */

                if (transferState == FileServerState.RECEIVE_SECTION) {

                    FileSegment segment = (FileSegment)asdu.GetElement (0);

                    logger ("Received F_SG_NA_1(segment) (NoS=" + segment.NameOfSection + ", LoS=" + segment.LengthOfSegment + ")");

                    if (fileReceiver != null) {
                        fileReceiver.SegmentReceived (segment.NameOfSection, currentSectionOffset, segment.LengthOfSegment, segment.SegmentData);
                    }

                    currentSectionOffset += segment.LengthOfSegment;
                } else {
                    logger ("Unexpected F_SG_NA_1(file segment)");
                }


                break;

            case TypeID.F_LS_NA_1: /* Last Segment/Section */

                logger ("Received F_LS_NA_1 (last segment/section)");

                if (transferState == FileServerState.RECEIVE_SECTION) {

                    FileLastSegmentOrSection lastSection = (FileLastSegmentOrSection)asdu.GetElement (0);

                    if (lastSection.LSQ == LastSectionOrSegmentQualifier.SECTION_TRANSFER_WITHOUT_DEACT) {

                        ASDU sectionAck = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                        sectionAck.AddInformationObject (new FileACK (ioa, nof, lastSection.NameOfSection, AcknowledgeQualifier.POS_ACK_SECTION, FileError.DEFAULT));

                        connection.SendASDU (sectionAck);
                        lastSentTime = SystemUtils.currentTimeMillis ();

                        logger ("Send section ACK");

                        transferState = FileServerState.WAITING_FOR_SECTION_READY;

                    } else if (lastSection.LSQ == LastSectionOrSegmentQualifier.FILE_TRANSFER_WITH_DEACT) {
                        /* master aborted transfer */

                        if (fileReceiver != null)
                            fileReceiver.Finished (FileErrorCode.ABORTED_BY_REMOTE);

                        transferState = FileServerState.UNSELECTED_IDLE;
                    } else {

                    }


                } else if (transferState == FileServerState.WAITING_FOR_SECTION_READY) {

                    FileLastSegmentOrSection lastSection = (FileLastSegmentOrSection)asdu.GetElement (0);

                    if (lastSection.LSQ == LastSectionOrSegmentQualifier.FILE_TRANSFER_WITHOUT_DEACT) {

                        ASDU fileAck = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                        fileAck.AddInformationObject(new FileACK (ioa, nof, lastSection.NameOfSection, AcknowledgeQualifier.POS_ACK_FILE, FileError.DEFAULT));

                        connection.SendASDU (fileAck);
                        lastSentTime = SystemUtils.currentTimeMillis ();

                        logger ("Send file ACK");

                        if (fileReceiver != null)
                            fileReceiver.Finished (FileErrorCode.SUCCESS);

                        transferState = FileServerState.UNSELECTED_IDLE;

                        logger ("Received file success");
                    }
                    else if (lastSection.LSQ == LastSectionOrSegmentQualifier.FILE_TRANSFER_WITH_DEACT) {
                         /* master aborted transfer */

                        if (fileReceiver != null)
                            fileReceiver.Finished (FileErrorCode.ABORTED_BY_REMOTE);

                        transferState = FileServerState.UNSELECTED_IDLE;
                    } else {
                        logger ("F_LS_NA_1 with unexpected LSQ: " + lastSection.LSQ.ToString ());
                    }
                }

                break;

            case TypeID.F_AF_NA_1: /*  124 - ACK file, ACK section */

                logger("Received file/section ACK F_AF_NA_1");

                //TODO move COT check to beginning of function!
                if (asdu.Cot == CauseOfTransmission.FILE_TRANSFER)
                {

                    if (transferState != FileServerState.UNSELECTED_IDLE)
                    {

                        IFileProvider file = selectedFile.provider;

                        FileACK ack = (FileACK)asdu.GetElement(0);

                        if (ack.AckQualifier == AcknowledgeQualifier.POS_ACK_FILE)
                        {

                            logger("Received positive file ACK");

                            if (transferState == FileServerState.WAITING_FOR_FILE_ACK)
                            {

                                selectedFile.provider.TransferComplete(true);
                                selectedFile.selectedBy = null;

                                availableFiles.RemoveFile(selectedFile.provider);

                                selectedFile = null;

                                transferState = FileServerState.UNSELECTED_IDLE;
                            }
                            else
                            {
                                logger("Unexpected file transfer state --> abort file transfer");

                                transferState = FileServerState.SEND_ABORT;
                            }


                        }
                        else if (ack.AckQualifier == AcknowledgeQualifier.NEG_ACK_FILE)
                        {

                            logger("Received negative file ACK - stop transfer");

                            if (transferState == FileServerState.WAITING_FOR_FILE_ACK)
                            {

                                selectedFile.provider.TransferComplete(false);

                                selectedFile.selectedBy = null;
                                selectedFile = null;

                                transferState = FileServerState.UNSELECTED_IDLE;
                            }
                            else
                            {
                                logger("Unexpected file transfer state --> abort file transfer");

                                transferState = FileServerState.SEND_ABORT;
                            }

                        }
                        else if (ack.AckQualifier == AcknowledgeQualifier.NEG_ACK_SECTION)
                        {

                            logger("Received negative file section ACK - repeat section");

                            if (transferState == FileServerState.WAITING_FOR_SECTION_ACK)
                            {
                                currentSectionOffset = 0;
                                sectionChecksum = 0;

                                ASDU sectionReady = new ASDU(alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, file.GetCA(), false);

                                sectionReady.AddInformationObject(
                                    new SectionReady(selectedFile.provider.GetIOA(), selectedFile.provider.GetNameOfFile(), currentSectionNumber, currentSectionSize, false));

                                connection.SendASDU(sectionReady);


                                transferState = FileServerState.TRANSMIT_SECTION;
                            }
                            else
                            {
                                logger("Unexpected file transfer state --> abort file transfer");

                                transferState = FileServerState.SEND_ABORT;
                            }

                        }
                        else if (ack.AckQualifier == AcknowledgeQualifier.POS_ACK_SECTION)
                        {

                            if (transferState == FileServerState.WAITING_FOR_SECTION_ACK)
                            {
                                currentSectionNumber++;

                                int nextSectionSize = 
                                    selectedFile.provider.GetSectionSize(currentSectionNumber - 1);

                                currentSectionOffset = 0;

                                ASDU responseAsdu = new ASDU(alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, file.GetCA(), false);

                                if (nextSectionSize == -1)
                                {
                                    logger("Received positive file section ACK - send last section indication");

                                    responseAsdu.AddInformationObject(
                                        new FileLastSegmentOrSection(file.GetIOA(), file.GetNameOfFile(), 
                                            (byte)currentSectionNumber, 
                                            LastSectionOrSegmentQualifier.FILE_TRANSFER_WITHOUT_DEACT,
                                            fileChecksum));

                                    transferState = FileServerState.WAITING_FOR_FILE_ACK;
                                }
                                else
                                {
                                    logger("Received positive file section ACK - send next section ready indication");

                                    currentSectionSize = nextSectionSize;

                                    responseAsdu.AddInformationObject(
                                        new SectionReady(selectedFile.provider.GetIOA(), selectedFile.provider.GetNameOfFile(), currentSectionNumber, currentSectionSize, false));

                                    transferState = FileServerState.WAITING_FOR_SECTION_CALL;
                                }

                                connection.SendASDU(responseAsdu);

                                lastSentTime = SystemUtils.currentTimeMillis ();

                                sectionChecksum = 0;
                            }
                            else
                            {
                                logger("Unexpected file transfer state --> abort file transfer");

                                transferState = FileServerState.SEND_ABORT;
                            }
                        }
                    }
                    else
                    {
                        // No file transmission in progress --> what to do?
                        logger("Unexpected File ACK message -> ignore");
                    }

                }
                else
                {
                    asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                    connection.SendASDU(asdu);
                }
                break;

            case TypeID.F_SC_NA_1: /* 122 - Call/Select directory/file/section */

                logger("Received call/select F_SC_NA_1");

                if (asdu.Cot == CauseOfTransmission.FILE_TRANSFER)
                {

                    FileCallOrSelect sc = (FileCallOrSelect)asdu.GetElement(0);


                    if (sc.SCQ == SelectAndCallQualifier.SELECT_FILE)
                    {

                        if (transferState == FileServerState.UNSELECTED_IDLE)
                        {

                            logger("Received SELECT FILE");

                            CS101n104File file = availableFiles.GetFile(asdu.Ca, sc.ObjectAddress, sc.NOF);

                            if (file == null)
                            {
                                asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                connection.SendASDU(asdu);
                            }
                            else
                            {

                                ASDU fileReady = new ASDU(alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                                // check if already selected
                                if (file.selectedBy == null)
                                {
                                    file.selectedBy = this;

                                    fileReady.AddInformationObject(new FileReady(sc.ObjectAddress, sc.NOF, file.provider.GetFileSize(), true));

                                    lastSentTime = SystemUtils.currentTimeMillis ();

                                    selectedFile = file;

                                    transferState = FileServerState.WAITING_FOR_FILE_CALL;

                                }
                                else
                                {
                                    fileReady.AddInformationObject(new FileReady(sc.ObjectAddress, sc.NOF, 0, false));

                                    transferState = FileServerState.UNSELECTED_IDLE;
                                }

                                connection.SendASDU(fileReady);


                            }

                        }
                        else
                        {
                            logger("Unexpected SELECT FILE message");
                        }

                    }
                    else if (sc.SCQ == SelectAndCallQualifier.DEACTIVATE_FILE)
                    {

                        logger("Received DEACTIVATE FILE");

                        if (transferState != FileServerState.UNSELECTED_IDLE)
                        {

                            if (selectedFile != null)
                            {
                                selectedFile.selectedBy = null;
                                selectedFile = null;
                            }

                            transferState = FileServerState.UNSELECTED_IDLE;

                        }
                        else
                        {
                            logger("Unexpected DEACTIVATE FILE message");
                        }

                    }
                    else if (sc.SCQ == SelectAndCallQualifier.REQUEST_FILE)
                    {

                        logger("Received CALL FILE");

                        if (transferState == FileServerState.WAITING_FOR_FILE_CALL)
                        {

                            if (selectedFile.provider.GetIOA() != sc.ObjectAddress)
                            {
                                logger("Unkown IOA");
                                asdu.IsNegative = true;
                                asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                connection.SendASDU(asdu);
                            }
                            else
                            {

                                ASDU sectionReady = new ASDU(alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                                currentSectionNumber = 1;
                                currentSectionOffset = 0;
                                currentSectionSize = selectedFile.provider.GetSectionSize (0);

                                sectionReady.AddInformationObject(new SectionReady(sc.ObjectAddress, selectedFile.provider.GetNameOfFile(), currentSectionNumber, currentSectionSize, false));

                                connection.SendASDU(sectionReady);

                                lastSentTime = SystemUtils.currentTimeMillis ();

                                logger ("Send SECTION READY");

                                transferState = FileServerState.WAITING_FOR_SECTION_CALL;
                            }

                        }
                        else
                        {
                            logger("Unexpected FILE CALL message");
                        }


                    }
                    else if (sc.SCQ == SelectAndCallQualifier.REQUEST_SECTION)
                    {

                        logger ("Received CALL SECTION (NoS=" + sc.NameOfSection + ") current section: " + currentSectionNumber);

                        if (transferState == FileServerState.WAITING_FOR_SECTION_CALL)
                        {

                            if (selectedFile.provider.GetIOA() != sc.ObjectAddress)
                            {
                                logger("Unkown IOA");
                                asdu.IsNegative = true;
                                asdu.Cot = CauseOfTransmission.UNKNOWN_INFORMATION_OBJECT_ADDRESS;
                                connection.SendASDU(asdu);
                            }
                            else
                            {
                                if (asdu.IsNegative) {

                                    currentSectionNumber++;
                                    currentSectionOffset = 0;

                                    currentSectionSize = selectedFile.provider.GetSectionSize (currentSectionNumber - 1);

                                    if (currentSectionSize > 0) {

                                        // send section ready with new section number

                                        ASDU sectionReady = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);

                                        currentSectionSize = selectedFile.provider.GetSectionSize (0);

                                        sectionReady.AddInformationObject (new SectionReady (sc.ObjectAddress, selectedFile.provider.GetNameOfFile (), currentSectionNumber, currentSectionSize, false));

                                        connection.SendASDU (sectionReady);

                                        lastSentTime = SystemUtils.currentTimeMillis ();

                                        logger ("Send F_SR_NA_1 (section ready) (NoS = " + currentSectionNumber + ")");

                                        transferState = FileServerState.WAITING_FOR_SECTION_CALL;

                                    } else {
                                        // send last section PDU

                                        ASDU lastSection = new ASDU (alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, asdu.Ca, false);


                                        lastSection.AddInformationObject (
                                               new FileLastSegmentOrSection (selectedFile.provider.GetIOA (), selectedFile.provider.GetNameOfFile (),
                                                (byte)currentSectionNumber,
                                                LastSectionOrSegmentQualifier.FILE_TRANSFER_WITHOUT_DEACT,
                                                fileChecksum));

                                        connection.SendASDU (lastSection);

                                        logger ("Send F_LS_NA_1 (last section))");

                                        lastSentTime = SystemUtils.currentTimeMillis ();

                                        transferState = FileServerState.WAITING_FOR_FILE_ACK;
                                    }

                                } 
                                else {
                                    currentSectionSize = selectedFile.provider.GetSectionSize (sc.NameOfSection - 1);

                                    if (currentSectionSize > 0) {
                                        currentSectionNumber = sc.NameOfSection;
                                        currentSectionOffset = 0;

                                        transferState = FileServerState.TRANSMIT_SECTION;
                                    } else {
                                        logger ("Unexpected number of section");
                                        logger ("Send negative confirm");
                                        asdu.IsNegative = true;

                                        lastSentTime = SystemUtils.currentTimeMillis ();

                                        connection.SendASDU (asdu);
                                    }
                                }

                                  
                            }
                        }
                        else
                        {
                            logger("Unexpected SECTION CALL message");
                        }
                    }

                }
                else if (asdu.Cot == CauseOfTransmission.REQUEST)
                {
                    logger("Call directory received");

                    availableFiles.SendDirectoy(connection, false);

                }
                else
                {
                    asdu.IsNegative = true;
                    asdu.Cot = CauseOfTransmission.UNKNOWN_CAUSE_OF_TRANSMISSION;
                    connection.SendASDU(asdu);
                }
                break;

            default:
                handled = false;
                break;
            }

            return handled;
        }

        public void HandleFileTransmission()
        {


            if (transferState != FileServerState.UNSELECTED_IDLE)
            {

                if (transferState == FileServerState.TRANSMIT_SECTION)
                {

                    if (selectedFile != null)
                    {

                        IFileProvider file = selectedFile.provider;

                        ASDU fileAsdu = new ASDU(alParameters, CauseOfTransmission.FILE_TRANSFER, false, false, 0, file.GetCA(), false);


                        if (currentSectionOffset == currentSectionSize)
                        {

                            // send last segment

                            fileAsdu.AddInformationObject(
                                new FileLastSegmentOrSection(file.GetIOA(), file.GetNameOfFile(), 
                                    currentSectionNumber, 
                                    LastSectionOrSegmentQualifier.SECTION_TRANSFER_WITHOUT_DEACT,
                                    sectionChecksum));

                            fileChecksum += sectionChecksum;
                            sectionChecksum = 0;


                            logger("Send LAST SEGMENT (NoS=" + currentSectionNumber + ")");

                            connection.SendASDU(fileAsdu);

                            lastSentTime = SystemUtils.currentTimeMillis ();

                            transferState = FileServerState.WAITING_FOR_SECTION_ACK;

                        }
                        else
                        {

                            int currentSegmentSize = currentSectionSize - currentSectionOffset;

                            if (currentSegmentSize > maxSegmentSize)
                                currentSegmentSize = maxSegmentSize;

                            byte[] segmentData = new byte[currentSegmentSize];

                            file.GetSegmentData(currentSectionNumber - 1,
                                currentSectionOffset,
                                currentSegmentSize,
                                segmentData);

                            fileAsdu.AddInformationObject(
                                new FileSegment(file.GetIOA(), file.GetNameOfFile(), currentSectionNumber, 
                                    segmentData));

                            byte checksum = 0;

                            foreach (byte octet in segmentData)
                            {
                                checksum += octet;
                            }

                            connection.SendASDU(fileAsdu);

                            lastSentTime = SystemUtils.currentTimeMillis ();

                            sectionChecksum += checksum;

                            logger("Send SEGMENT (NoS=" + currentSectionNumber +  ", CHS=" + sectionChecksum + ")");
                            currentSectionOffset += currentSegmentSize;

                        }
                    }
                }

                // check for timeout
                if (SystemUtils.currentTimeMillis () > lastSentTime + timeout) {
                    logger ("Abort file transfer due to timeout");

                    if (selectedFile != null) {
                        selectedFile.selectedBy = null;
                        selectedFile = null;
                    }

                    transferState = FileServerState.UNSELECTED_IDLE;
                }

            }
        }
    }

}