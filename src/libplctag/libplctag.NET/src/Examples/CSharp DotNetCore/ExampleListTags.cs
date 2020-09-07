using System;
using System.Linq;
using System.Net;
using System.Text;
using ConsoleTables;
using libplctag;
using libplctag.DataTypes;

namespace CSharpDotNetCore
{
    static class ExampleListTags
    {
        public static void Run()
        {
            //This example will list all tags at the controller-scoped level

            var tags = new Tag<TagInfoPlcMapper, TagInfo[]>()
            {
                Gateway = "10.10.10.10",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip,
                Name = "@tags"
            };

            tags.Read();

            ConsoleTable
                .From(tags.Value.Select(t => new
                {
                    t.Id,
                    Type = $"0x{t.Type:X}",
                    t.Name,
                    t.Length,
                    Dimensions = string.Join(',', t.Dimensions)
                }))
                .Configure(o => o.NumberAlignment = Alignment.Right)
                .Write(Format.Default);

        }

    }

}