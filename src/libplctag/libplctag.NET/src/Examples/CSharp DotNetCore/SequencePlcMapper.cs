using libplctag;
using libplctag.DataTypes;
using System;
using System.Collections;

namespace CSharpDotNetCore
{


    /// <summary>
    /// This is an example plcMapper for a User Defined Type (UDT)
    /// </summary>
    /// <remarks>
    /// 
    /// This type was developed for use on a CompactLogix controller
    /// and is included to show how to develop a custom plcMapper
    /// 
    /// Step_No     DINT            4 bytes
    /// Next_Step   DINT            4 bytes
    /// Command     DINT            4 bytes
    /// Idle_Step   DINT            4 bytes
    /// Fault_Step  DINT            4 bytes
    /// Init_Step   DINT            4 bytes
    /// Stop        BOOL            \
    /// Hold        BOOL            |-- packed into 4 bytes
    /// Fault       BOOL            /
    /// Timer       TIMER[20]       12 bytes x 20 = 240 bytes
    /// ------------------------------------------------------
    /// Total size = 268 bytes
    /// 
    /// 
    /// In order to understand how the structure is encoded
    /// we need to inspect the underlying buffer.
    /// Use GetBuffer() from the tag object 
    /// to access to the raw bytes. Then, use your PLC
    /// programming software to manually modify values and see
    /// how the raw bytes change.
    /// 
    /// If you are accessing an array of your UDT, try starting
    /// out by addressing only one element of this array (i.e.
    /// set ElementCount = 1).
    /// 
    /// </remarks>
    public class SequencePlcMapper : PlcMapperBase<Sequence>, IPlcMapper<Sequence>, IPlcMapper<Sequence[]>
    {



        // Because our UDT has an unchanging ElementSize,
        // provide the value so the tag constructor can use it
        // If ElementSize = null, this will not be passed to the
        // Tag constructor
        public override int? ElementSize => 268;



        // This function is used to decode the binary buffer
        // into a CLR data transfer object
        // The function is called once per array element, so we only 
        // need to decode one array element at a time.
        override public Sequence Decode(Tag tag, int offset)
        {



            // If our UDT has a size that does not change, we can set this based on ElementSize
            // Some types have an ElementSize that varies with it's contents (e.g. STRING on some controllers)
            // Those types must wait until they know the actual elementSize before returning it
            //elementSize = ElementSize.Value;



            // Plain DINT objects
            //
            // Note that the buffer access is always offset
            // This is so that our PlcMapper can be used in both
            // Single Values or Arrays
            var DINT0 = tag.GetInt32(offset + 0);
            var DINT1 = tag.GetInt32(offset + 4);
            var DINT2 = tag.GetInt32(offset + 8);
            var DINT3 = tag.GetInt32(offset + 12);
            var DINT4 = tag.GetInt32(offset + 16);
            var DINT5 = tag.GetInt32(offset + 20);



            // Our BOOLs are packed into this object.
            // I've chosen to make use of the BitArray class
            // which takes an integer array or byte array
            var PACKED_BOOLS = tag.GetInt32(offset + 24);
            var bools = new BitArray(new int[] { PACKED_BOOLS });



            // We can make use of other PlcMappers!
            // This means that if our UDT contains other structures (or UDTs)
            var timerPlcMapper = new TimerPlcMapper()
            { PlcType = this.PlcType };                     // Pass the PlcType through to this PlcMapper just in case it's behaviour depends on PlcType


            var TIMERS = new AbTimer[20];
            for (int ii = 0; ii < 20; ii++)
            {
                var timerOffset = offset + 28 + ii * timerPlcMapper.ElementSize.Value;
                TIMERS[ii] = timerPlcMapper.Decode(tag, timerOffset);
            }



            // We now have all of our objects Decoded
            // and can instantiate our Plain Old Class Object (POCO)
            // With the appropriate values
            return new Sequence()
            {
                Step_No = DINT0,
                Next_Step = DINT1,
                Command = DINT2,
                Idle_Step = DINT3,
                Fault_Step = DINT4,
                Init_Step = DINT5,
                Stop = bools[0],
                Hold = bools[1],
                Fault = bools[2],
                Timer = TIMERS
            };

        }



        override public void Encode(Tag tag, int offset, Sequence value)
        {

            var DINT0 = value.Step_No;
            var DINT1 = value.Next_Step;
            var DINT2 = value.Command;
            var DINT3 = value.Idle_Step;
            var DINT4 = value.Fault_Step;
            var DINT5 = value.Init_Step;

            var bools = new BitArray(32);
            bools[0] = value.Stop;
            bools[1] = value.Hold;
            bools[2] = value.Fault;

            var DINT6 = BitArrayToInt(bools);

            tag.SetInt32(offset + 0, DINT0);
            tag.SetInt32(offset + 4, DINT1);
            tag.SetInt32(offset + 8, DINT2);
            tag.SetInt32(offset + 12, DINT3);
            tag.SetInt32(offset + 16, DINT4);
            tag.SetInt32(offset + 20, DINT5);
            tag.SetInt32(offset + 24, DINT6);

            var timerPlcMapper = new TimerPlcMapper();
            for (int ii = 0; ii < 20; ii++)
            {
                var timerOffset = offset + 28 + ii * timerPlcMapper.ElementSize.Value;
                timerPlcMapper.Encode(tag, timerOffset, value.Timer[ii]);
            }

        }

        static int BitArrayToInt(BitArray binary)
        {
            if (binary == null)
                throw new ArgumentNullException(nameof(binary));
            if (binary.Length > 32)
                throw new ArgumentException("Must be at most 32 bits long");

            var result = new int[1];
            binary.CopyTo(result, 0);
            return result[0];
        }
    }



    /// <summary>
    /// Data Transfer Object for the User Defined Type.
    /// </summary>
    /// 
    /// <remarks>
    /// Although it is not absolutely required, it is best
    /// practice to here use the same naming and casing as is used
    /// in the User Defined Type in the PLC, and to keep these
    /// classes as pure Data Transfer Objects.
    /// </remarks>
    /// 
    public class Sequence
    {
        public int Command { get; set; }
        public bool Fault { get; set; }
        public int Fault_Step { get; set; }
        public bool Hold { get; set; }
        public int Idle_Step { get; set; }
        public int Next_Step { get; set; }
        public int Init_Step { get; set; }
        public int Step_No { get; set; }
        public bool Stop { get; set; }
        public AbTimer[] Timer { get; set; }
    }


}
