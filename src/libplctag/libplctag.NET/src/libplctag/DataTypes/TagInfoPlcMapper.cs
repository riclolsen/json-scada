using System;
using System.Collections.Generic;
using System.Linq;
using System.Text;

namespace libplctag.DataTypes
{

    public class TagInfo
    {
        public uint Id { get; set; }
        public int Type { get; set; }
        public string Name { get; set; }
        public int Length { get; set; }
        public uint[] Dimensions { get; set; }
    }

    public class TagInfoPlcMapper : IPlcMapper<TagInfo[]>
    {

        const int TAG_STRING_SIZE = 200;

        public PlcType PlcType { get; set; }

        //TODO: Is null appropriate since it's unknown?
        public int? ElementSize => null;

        public int[] ArrayDimensions { get => null; set => throw new NotImplementedException("This plcMapper can only be used to read Tag Information");
        }

        public TagInfo Decode(Tag tag, int offset, out int elementSize)
        {

            var tagInstanceId = tag.GetUInt32(offset);
            var tagType = tag.GetUInt16(offset + 4);
            var tagLength = tag.GetUInt16(offset + 6);
            var tagArrayDims = new uint[]
            {
                tag.GetUInt32(offset + 8),
                tag.GetUInt32(offset + 12),
                tag.GetUInt32(offset + 16)
            };

            var apparentTagNameLength = (int)tag.GetUInt16(offset + 20);
            var actualTagNameLength = Math.Min(apparentTagNameLength, TAG_STRING_SIZE * 2 - 1);

            var tagNameBytes = Enumerable.Range(offset + 22, actualTagNameLength)
                .Select(o => tag.GetUInt8(o))
                .Select(Convert.ToByte)
                .ToArray();

            var tagName = Encoding.ASCII.GetString(tagNameBytes);

            elementSize = 22 + actualTagNameLength;

            return new TagInfo()
            {
                Id = tagInstanceId,
                Type = tagType,
                Name = tagName,
                Length = tagLength,
                Dimensions = tagArrayDims
            };

        }

        public TagInfo[] Decode(Tag tag)
        {
            var buffer = new List<TagInfo>();

            var tagSize = tag.GetSize();

            int offset = 0;
            while (offset < tagSize)
            {
                buffer.Add(Decode(tag, offset, out int elementSize));
                offset += elementSize;
            }

            return buffer.ToArray();
        }

        public void Encode(Tag tag, TagInfo[] value)
        {
            throw new NotImplementedException("This plcMapper can only be used to read Tag Information");
        }

        public int? GetElementCount()
        {
            //TODO: We know this value after we decode once. SHould we trigger a decode or cache the value after first decode?
            return null;
        }
    }

}
