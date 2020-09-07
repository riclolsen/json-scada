using libplctag;
using libplctag.DataTypes;
using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Text;

namespace CSharpDotNetCore
{
    class ExampleDatatypes
    {
        const string gateway = "10.10.10.10";
        const string path = "1,0";


        public static void Run()
        {

            //Bool
            var boolTag = new Tag<BoolPlcMapper, bool>()
            {
                Name = "TestBOOL",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            boolTag.Read();


            //Signed Numbers
            var sintTag = new Tag<SintPlcMapper, sbyte>()
            {
                Name = "TestSINT",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            sintTag.Read();

            var intTag = new Tag<IntPlcMapper, short>()
            {
                Name = "TestINT",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            intTag.Read();


            var dintTag = new Tag<DintPlcMapper, int>()
            {
                Name = "TestBOOL",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            dintTag.Read();


            var lintTag = new Tag<LintPlcMapper, long>()
            {
                Name = "TestLINT",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            lintTag.Read();


            //Floating Points
            var realTag = new Tag<RealPlcMapper, float>()
            {
                Name = "TestREAL",
                Gateway = gateway,
                Path = path,
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip
            };
            realTag.Read();


            //Strings and String Arrays
            var stringTag = new Tag<StringPlcMapper, string[]>()
            {
                Name = "MY_STRING_1D[0]",
                Gateway = gateway,
                Path = path,
                Protocol = Protocol.ab_eip,
                PlcType = PlcType.ControlLogix,
                ArrayDimensions = new int[] { 100 },
            };
            var r = new Random((int)DateTime.Now.ToBinary());

            for (int ii = 0; ii < stringTag.Value.Length; ii++)
                stringTag.Value[ii] = r.Next().ToString();

            stringTag.Write();

            Console.WriteLine("DONE");

        }

    }

}
