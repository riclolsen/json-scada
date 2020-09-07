namespace libplctag.DataTypes
{
    public class IntPlcMapper : PlcMapperBase<short>, IPlcMapper<short>, IPlcMapper<short[]>
    {
        public override int? ElementSize => 2;

        override public short Decode(Tag tag, int offset) => tag.GetInt16(offset);

        override public void Encode(Tag tag, int offset, short value) => tag.SetInt16(offset, value);

    }
}
