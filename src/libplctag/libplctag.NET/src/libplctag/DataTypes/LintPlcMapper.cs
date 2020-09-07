namespace libplctag.DataTypes
{
    public class LintPlcMapper : PlcMapperBase<long>, IPlcMapper<long>, IPlcMapper<long[]>
    {
        public override int? ElementSize => 8;

        override public long Decode(Tag tag, int offset) => tag.GetInt64(offset);

        override public void Encode(Tag tag, int offset, long value) => tag.SetInt64(offset, value);

    }
}
