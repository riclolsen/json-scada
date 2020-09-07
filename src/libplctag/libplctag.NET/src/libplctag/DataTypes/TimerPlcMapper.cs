using System;
using System.Collections;
using System.Collections.Generic;
using System.Text;

namespace libplctag.DataTypes
{
    public class TimerPlcMapper : PlcMapperBase<AbTimer>, IPlcMapper<AbTimer>, IPlcMapper<AbTimer[]>
    {

        public override int? ElementSize => 12;

        public override AbTimer Decode(Tag tag, int offset)
        {

            // Needed to look at RsLogix documentation for structure of TIMER
            var DINT2 = tag.GetInt32(offset);
            var DINT1 = tag.GetInt32(offset + 4);
            var DINT0 = tag.GetInt32(offset + 8);

            // The third DINT packs a few BOOLs into it
            var bitArray = new BitArray(new int[] { DINT2 });

            var timer = new AbTimer
            {
                Accumulated = DINT0,         // ACC
                Preset = DINT1,              // PRE
                Done = bitArray[29],         // DN
                InProgress = bitArray[30],   // TT
                Enabled = bitArray[31]       // EN
            };

            return timer;

        }

        public override void Encode(Tag tag, int offset, AbTimer value)
        {
            var DINT0 = value.Accumulated;
            var DINT1 = value.Preset;

            var asdf = new BitArray(32);
            asdf[29] = value.Done;
            asdf[30] = value.InProgress;
            asdf[31] = value.Enabled;
            var DINT2 = BitArrayToInt(asdf);

            tag.SetInt32(offset, DINT2);
            tag.SetInt32(offset + 4, DINT1);
            tag.SetInt32(offset + 8, DINT0);

        }

        static int BitArrayToInt(BitArray binary)
        {
            if (binary == null)
                throw new ArgumentNullException("binary");
            if (binary.Length > 32)
                throw new ArgumentException("Must be at most 32 bits long");

            var result = new int[1];
            binary.CopyTo(result, 0);
            return result[0];
        }
    }

    public class AbTimer
    {
        public int Preset { get; set; }
        public int Accumulated { get; set; }
        public bool Enabled { get; set; }
        public bool InProgress { get; set; }
        public bool Done { get; set; }
    }
}
