/*
 *  MeasuredValueScaled.cs
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

namespace lib60870.CS101
{
    public class MeasuredValueScaled : InformationObject
    {
        override public int GetEncodedSize()
        {
            return 3;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.M_ME_NB_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return true;
            }
        }

        private ScaledValue scaledValue;

        public ScaledValue ScaledValue
        {
            get
            {
                return scaledValue;
            }
        }

        private QualityDescriptor quality;

        public QualityDescriptor Quality
        {
            get
            {
                return quality;
            }
        }

        /// <summary>
        /// Initializes a new instance of the <see cref="lib60870.MeasuredValueScaled"/> class.
        /// </summary>
        /// <param name="objectAddress">Information object address</param>
        /// <param name="value">scaled value (range -32768 - 32767) </param>
        /// <param name="quality">quality descriptor (according to IEC 60870-5-101:2003 7.2.6.3)</param>
        public MeasuredValueScaled(int objectAddress, int value, QualityDescriptor quality)
            : base(objectAddress)
        {
            scaledValue = new ScaledValue(value);
            this.quality = quality;
        }

        internal MeasuredValueScaled(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSquence)
            : base(parameters, msg, startIndex, isSquence)
        {
            if (!isSquence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            scaledValue = new ScaledValue(msg, startIndex);
            startIndex += 2;

            /* parse QDS (quality) */
            quality = new QualityDescriptor(msg[startIndex++]);
        }

        public MeasuredValueScaled(MeasuredValueScaled original)
            : base(original.ObjectAddress)
        {
            scaledValue = new ScaledValue(original.ScaledValue);
            quality = new QualityDescriptor(original.quality);
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.AppendBytes(scaledValue.GetEncodedValue());

            frame.SetNextByte(quality.EncodedValue);
        }

    }

    public class MeasuredValueScaledWithCP24Time2a : MeasuredValueScaled
    {
        override public int GetEncodedSize()
        {
            return 6;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.M_ME_TB_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private CP24Time2a timestamp;

        public CP24Time2a Timestamp
        {
            get
            {
                return timestamp;
            }
        }

        public MeasuredValueScaledWithCP24Time2a(int objectAddress, int value, QualityDescriptor quality, CP24Time2a timestamp)
            : base(objectAddress, value, quality)
        {
            this.timestamp = timestamp;
        }

        public MeasuredValueScaledWithCP24Time2a(MeasuredValueScaledWithCP24Time2a original)
            : base(original)
        {
            timestamp = new CP24Time2a(timestamp);
        }

        internal MeasuredValueScaledWithCP24Time2a(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            startIndex += 3; /* scaledValue + QDS */

            /* parse CP56Time2a (time stamp) */
            timestamp = new CP24Time2a(msg, startIndex);
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.AppendBytes(timestamp.GetEncodedValue());
        }

    }

    public class MeasuredValueScaledWithCP56Time2a : MeasuredValueScaled
    {
        override public int GetEncodedSize()
        {
            return 10;
        }

        override public TypeID Type
        {
            get
            {
                return TypeID.M_ME_TE_1;
            }
        }

        override public bool SupportsSequence
        {
            get
            {
                return false;
            }
        }

        private CP56Time2a timestamp;

        public CP56Time2a Timestamp
        {
            get
            {
                return timestamp;
            }
        }

        public MeasuredValueScaledWithCP56Time2a(int objectAddress, int value, QualityDescriptor quality, CP56Time2a timestamp)
            : base(objectAddress, value, quality)
        {
            this.timestamp = timestamp;
        }

        public MeasuredValueScaledWithCP56Time2a(MeasuredValueScaledWithCP56Time2a original)
            : base(original)
        {
            timestamp = new CP56Time2a(original.timestamp);
        }

        internal MeasuredValueScaledWithCP56Time2a(ApplicationLayerParameters parameters, byte[] msg, int startIndex, bool isSequence)
            : base(parameters, msg, startIndex, isSequence)
        {
            if (!isSequence)
                startIndex += parameters.SizeOfIOA; /* skip IOA */

            if ((msg.Length - startIndex) < GetEncodedSize())
                throw new ASDUParsingException("Message too small");

            startIndex += 3; /* scaledValue + QDS */

            /* parse CP56Time2a (time stamp) */
            timestamp = new CP56Time2a(msg, startIndex);
        }

        public override void Encode(Frame frame, ApplicationLayerParameters parameters, bool isSequence)
        {
            base.Encode(frame, parameters, isSequence);

            frame.AppendBytes(timestamp.GetEncodedValue());
        }

    }

}

