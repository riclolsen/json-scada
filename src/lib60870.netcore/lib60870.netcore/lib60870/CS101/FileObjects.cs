/*
 *  FileObjects.cs
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
using System.Collections.Generic;

namespace lib60870.CS101
{
    /// <summary>
    /// Name of file (NOF) - describes the type of a file
    /// </summary>
    public enum NameOfFile : ushort
    {
        DEFAULT = 0,
        TRANSPARENT_FILE = 1,
        DISTURBANCE_DATA = 2,
        SEQUENCES_OF_EVENTS = 3,
        SEQUENCES_OF_ANALOGUE_VALUES = 4
    }

    /// <summary>
    /// Select and call qualifier (SCQ)
    /// </summary>
    public enum SelectAndCallQualifier : byte
    {
        DEFAULT = 0,
        SELECT_FILE = 1,
        REQUEST_FILE = 2,
        DEACTIVATE_FILE = 3,
        DELETE_FILE = 4,
        SELECT_SECTION = 5,
        REQUEST_SECTION = 6,
        DEACTIVATE_SECTION = 7
    }

    /// <summary>
    /// Last section or segment qualifier (LSQ)
    /// </summary>
    public enum LastSectionOrSegmentQualifier : byte
    {
        NOT_USED = 0,
        FILE_TRANSFER_WITHOUT_DEACT = 1,
        FILE_TRANSFER_WITH_DEACT = 2,
        SECTION_TRANSFER_WITHOUT_DEACT = 3,
        SECTION_TRANSFER_WITH_DEACT = 4
    }

    /// <summary>
    /// Acknowledge qualifier (AFQ)
    /// </summary>
    public enum AcknowledgeQualifier
    {
        NOT_USED = 0,
        POS_ACK_FILE = 1,
        NEG_ACK_FILE = 2,
        POS_ACK_SECTION = 3,
        NEG_ACK_SECTION = 4
    }

    /// <summary>
    /// File Error
    /// </summary>
    public enum FileError
    {
        DEFAULT = 0,
        /* no error */
        REQ_MEMORY_NOT_AVAILABLE = 1,
        CHECKSUM_FAILED = 2,
        UNEXPECTED_COMM_SERVICE = 3,
        UNEXPECTED_NAME_OF_FILE = 4,
        UNEXPECTED_NAME_OF_SECTION = 5
    }

    /// <summary>
    /// File ready - F_FR_NA_1 (120)
    /// </summary>
    public class FileReady : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 6;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_FR_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;

        private int lengthOfFile;

        private byte frq;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public int LengthOfFile
        {
            get
            {
                return this.lengthOfFile;
            }
            set
            {
                lengthOfFile = value;
            }
        }

        /// <summary>
        /// Gets or sets the FRQ (File ready qualifier)
        /// </summary>
        public byte FRQ
        {
            get
            {
                return this.frq;
            }
            set
            {
                frq = value;
            }
        }

        public bool Positive
        {
            get { return ((frq & 0x80) == 0); }
        }

        internal FileReady(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            lengthOfFile = msg[startIndex++];
            lengthOfFile += (msg[startIndex++] * 0x100);
            lengthOfFile += (msg[startIndex++] * 0x10000);

            /* parse FRQ (file ready qualifier) */
            frq = msg[startIndex++];
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.CS101.FileReady"/> class.
        /// </summary>
        /// <param name="objectAddress">information object address (IOA)</param>
        /// <param name="nof">NOF (file type)</param>
        /// <param name="lengthOfFile">Length of file.</param>
        /// <param name="positive">If set to <c>true</c> positive ACK.</param>
        public FileReady(int objectAddress, NameOfFile nof, int lengthOfFile, bool positive)
            : base(objectAddress)
        {
            this.nof = nof;
            this.lengthOfFile = lengthOfFile;

            if (positive)
                frq = 0x00;
            else
                frq = 0x80;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte((byte)(lengthOfFile % 0x100));
            frame.SetNextByte((byte)((lengthOfFile / 0x100) % 0x100));
            frame.SetNextByte((byte)((lengthOfFile / 0x10000) % 0x100));

            frame.SetNextByte(frq);
        }
    }

    /// <summary>
    /// Section ready - F_SR_NA_1 (121)
    /// </summary>
    public class SectionReady : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 7;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_SR_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;
        private byte nameOfSection;

        private int lengthOfSection;

        private byte srq;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public byte NameOfSection
        {
            get
            {
                return this.nameOfSection;
            }
            set
            {
                nameOfSection = value;
            }
        }

        public int LengthOfSection
        {
            get
            {
                return this.lengthOfSection;
            }
            set
            {
                lengthOfSection = value;
            }
        }

        /// <summary>
        /// Gets or sets the SRQ (section ready qualifier)
        /// </summary>
        /// <value>The SRQ</value>
        public byte SRQ
        {
            get
            {
                return this.srq;
            }
            set
            {
                srq = value;
            }
        }

        public bool NotReady
        {
            get { return ((srq & 0x80) == 0x80); }
        }

        internal SectionReady(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            nameOfSection = msg[startIndex++];

            lengthOfSection = msg[startIndex++];
            lengthOfSection += (msg[startIndex++] * 0x100);
            lengthOfSection += (msg[startIndex++] * 0x10000);

            /* parse SRQ (section read qualifier) */
            srq = msg[startIndex++];
        }

        public SectionReady(int objectAddress, NameOfFile nof, byte nameOfSection, int lengthOfSection, bool notReady)
            : base(objectAddress)
        {
            this.nof = nof;
            this.nameOfSection = nameOfSection;
            this.lengthOfSection = lengthOfSection;

            if (notReady)
                srq = 0x80;
            else
                srq = 0;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte(nameOfSection);

            frame.SetNextByte((byte)(lengthOfSection % 0x100));
            frame.SetNextByte((byte)((lengthOfSection / 0x100) % 0x100));
            frame.SetNextByte((byte)((lengthOfSection / 0x10000) % 0x100));

            frame.SetNextByte(srq);
        }
    }

    /// <summary>
    /// Call/Select directory/file/section - F_SC_NA_1 (122)
    /// </summary>
    public class FileCallOrSelect : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 4;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_SC_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;
        private byte nameOfSection;

        private SelectAndCallQualifier scq;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public byte NameOfSection
        {
            get
            {
                return this.nameOfSection;
            }
            set
            {
                nameOfSection = value;
            }
        }

        /// <summary>
        /// Gets or sets the SCQ (select and call qualifier)
        /// </summary>
        /// <value>The SCQ</value>
        public SelectAndCallQualifier SCQ
        {
            get
            {
                return this.scq;
            }
            set
            {
                scq = value;
            }
        }

        internal FileCallOrSelect(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            nameOfSection = msg[startIndex++];

            /* parse SCQ (select and call qualifier) */
            scq = (SelectAndCallQualifier)msg[startIndex++];
        }

        public FileCallOrSelect(int objectAddress, NameOfFile nof, byte nameOfSection, SelectAndCallQualifier scq)
            : base(objectAddress)
        {
            this.nof = nof;
            this.nameOfSection = nameOfSection;
            this.scq = scq;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte(nameOfSection);

            frame.SetNextByte((byte)scq);
        }
    }

    /// <summary>
    /// Last segment/section - F_LS_NA_1 (123)
    /// </summary>
    public class FileLastSegmentOrSection : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 5;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_LS_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;
        private byte nameOfSection;

        private LastSectionOrSegmentQualifier lsq;

        private byte chs;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public byte NameOfSection
        {
            get
            {
                return this.nameOfSection;
            }
            set
            {
                nameOfSection = value;
            }
        }

        public LastSectionOrSegmentQualifier LSQ
        {
            get
            {
                return this.lsq;
            }
            set
            {
                lsq = value;
            }
        }

        /// <summary>
        /// Gets or sets the checksum
        /// </summary>
        /// <value>The checksum value</value>
        public byte CHS
        {
            get
            {
                return this.chs;
            }
            set
            {
                chs = value;
            }
        }

        internal FileLastSegmentOrSection(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            nameOfSection = msg[startIndex++];

            /* parse LSQ (last section or segment qualifier) */
            lsq = (LastSectionOrSegmentQualifier)msg[startIndex++];

            chs = msg[startIndex++];
        }

        public FileLastSegmentOrSection(int objectAddress, NameOfFile nof, byte nameOfSection, LastSectionOrSegmentQualifier lsq, byte checksum)
            : base(objectAddress)
        {
            this.nof = nof;
            this.nameOfSection = nameOfSection;
            this.lsq = lsq;
            this.chs = checksum;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte(nameOfSection);

            frame.SetNextByte((byte)lsq);
            frame.SetNextByte(chs);
        }
    }

    /// <summary>
    /// ACK file/section - F_AF_NA_1 (124)
    /// </summary>
    public class FileACK : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 4;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_AF_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;
        private byte nameOfSection;

        private byte afq;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public byte NameOfSection
        {
            get
            {
                return this.nameOfSection;
            }
            set
            {
                nameOfSection = value;
            }
        }

        /// <summary>
        /// Gets or sets the AFQ (acknowledge file or section qualifier)
        /// </summary>
        /// <value>The AFQ</value>
        public AcknowledgeQualifier AckQualifier
        {
            get
            {
                return  (AcknowledgeQualifier)(this.afq & 0x0f);
            }
            set
            {
                afq = (byte)(afq & 0xf0);
                afq += (byte)value;
            }
        }

        public FileError ErrorCode
        {
            get
            {
                return (FileError)(afq / 0x10);
            }
            set
            {
                afq = (byte)(afq & 0x0f);
                afq += (byte)((int)value * 0x10);
            }
        }

        public byte AFQ
        {
            get
            {
                return afq;
            }
            set
            {
                afq = value;
            }
        }

        internal FileACK(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            nameOfSection = msg[startIndex++];

            /* parse AFQ (acknowledge file or section qualifier) */
            afq = msg[startIndex++];
        }

        public FileACK(int objectAddress, NameOfFile nof, byte nameOfSection, AcknowledgeQualifier qualifier, FileError errorCode)
            : base(objectAddress)
        {
            this.nof = nof;
            this.nameOfSection = nameOfSection;
            this.AckQualifier = qualifier;
            this.ErrorCode = errorCode;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte(nameOfSection);

            frame.SetNextByte(afq);
        }
    }

    /// <summary>
    /// File directory - F_DR_TA_1 (126)
    /// </summary>
    public class FileDirectory : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 13;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_DR_TA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return true;
            }
        }

        private NameOfFile nof;
        private int lengthOfFile;
        private byte sof;
        /* Status of file (7.2.6.38) */

        private CP56Time2a creationTime;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public int LengthOfFile
        {
            get
            {
                return this.lengthOfFile;
            }
            set
            {
                this.lengthOfFile = value;
            }
        }

        /// <summary>
        /// Gets or sets the SOF (Status of file - 7.2.6.38 in IEC 60870-5-101:2003)
        /// </summary>
        /// <value>SOF value</value>
        public byte SOF
        {
            get
            {
                return this.sof;
            }
            set
            {
                sof = value;
            }
        }

        public int STATUS
        {
            get
            {
                return (int)(sof & 0x1f);
            }
        }

        public bool LFD
        {
            get
            {
                return ((sof & 0x20) == 0x20);
            }
        }

        public bool FOR
        {
            get
            {
                return ((sof & 0x40) == 0x40);
            }
        }

        public bool FA
        {
            get
            {
                return ((sof & 0x80) == 0x80);
            }
        }

        internal FileDirectory(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            lengthOfFile = msg[startIndex++];
            lengthOfFile += (msg[startIndex++] * 0x100);
            lengthOfFile += (msg[startIndex++] * 0x10000);

            sof = msg[startIndex++];

            /* parse CP56Time2a (creation time of file) */
            creationTime = new CP56Time2a(msg, startIndex);
        }

        public FileDirectory(int objectAddress, NameOfFile nof, int lengthOfFile, byte sof, CP56Time2a creationTime)
            : base(objectAddress)
        {
            this.nof = nof;
            this.lengthOfFile = lengthOfFile;
            this.sof = sof;
            this.creationTime = creationTime;
        }

        public FileDirectory(int objectAddress, NameOfFile nof, int lengthOfFile, int status, bool LFD, bool FOR, bool FA, CP56Time2a creationTime)
            : base(objectAddress)
        {
            this.nof = nof;
            this.lengthOfFile = lengthOfFile;

            if (status < 0)
                status = 0;
            else if (status > 31)
                status = 31;

            byte sof = (byte)status;

            if (LFD)
                sof += 0x20;

            if (FOR)
                sof += 0x40;

            if (FA)
                sof += 0x80;

            this.sof = sof;
            this.creationTime = creationTime;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte((byte)(lengthOfFile % 0x100));
            frame.SetNextByte((byte)((lengthOfFile / 0x100) % 0x100));
            frame.SetNextByte((byte)((lengthOfFile / 0x10000) % 0x100));

            frame.SetNextByte((byte)sof);

            frame.AppendBytes(creationTime.GetEncodedValue());
        }
    }

    /// <summary>
    /// File segment - F_SG_NA_1 (125)
    /// </summary>
    public class FileSegment : InformationObject
    {
        private static int ENCODED_SIZE = 4;

        override public int GetEncodedSize()
        {
            return ENCODED_SIZE;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.F_SG_NA_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private NameOfFile nof;
        private byte nameOfSection;

        private byte los;
        /* length of Segment */

        private byte[] data = null;

        public NameOfFile NOF
        {
            get
            {
                return this.nof;
            }
            set
            {
                nof = value;
            }
        }

        public byte NameOfSection
        {
            get
            {
                return this.nameOfSection;
            }
            set
            {
                nameOfSection = value;
            }
        }

        public byte LengthOfSegment
        {
            get
            {
                return los;
            }
            set
            {
                los = value;
            }
        }

        public byte[] SegmentData
        {
            get
            {
                return data;
            }
        }

        internal FileSegment(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            int nofValue;

            nofValue = msg[startIndex++];
            nofValue += (msg[startIndex++] * 0x100);

            nof = (NameOfFile)nofValue;

            nameOfSection = msg[startIndex++];

            los = msg[startIndex++];

            if (los > GetMaxDataSize(parameters))
                throw new ASDUParsingException("Payload data too large");

            if ((msg.Length - startIndex) < los)
                throw new ASDUParsingException("Message too small");

            data = new byte[los];

            for (int i = 0; i < los; i++)
                data[i] = msg[startIndex++];
				
        }

        public FileSegment(int objectAddress, NameOfFile nof, byte nameOfSection, byte[] data)
            : base(objectAddress)
        {
            this.nof = nof;
            this.nameOfSection = nameOfSection;
            this.data = data;
            this.los = (byte)data.Length;
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.SetNextByte((byte)((int)nof % 256));
            frame.SetNextByte((byte)((int)nof / 256));

            frame.SetNextByte(nameOfSection);

            frame.SetNextByte(los);

            if (data.Length > GetMaxDataSize(parameters))
                throw new ASDUParsingException("Payload data too large");

            frame.AppendBytes(data);
        }


        public static int GetMaxDataSize(ApplicationLayerParameters parameters)
        {
            int maxSize = parameters.MaxAsduLength
                 - parameters.SizeOfTypeId
                 - parameters.SizeOfVSQ
                 - parameters.SizeOfCA
                 - parameters.SizeOfCOT
                 - parameters.SizeOfIOA
                 - ENCODED_SIZE;
			
            return maxSize;
        }
    }
}
