using libplctag;
using libplctag.DataTypes;
using System;
using System.Net;
using System.Threading;

namespace CSharpDotNetFramework
{
    class ExampleRW
    {
        public static void Run()
        {
            Console.WriteLine($"\r\n*** ExampleRW ***");

            const int TIMEOUT = 5000;

            //DINT Test Read/Write
            var myTag = new Tag<DintPlcMapper, int>()
            {
                //Name of tag on the PLC, Controller-scoped would be just "SomeDINT"
                Name = "PROGRAM:SomeProgram.SomeDINT",
                
                //PLC IP Address
                Gateway = "10.10.10.10",

                //CIP path to PLC CPU. "1,0" will be used for most AB PLCs
                Path = "1,0", 

                //Type of PLC
                PlcType = PlcType.ControlLogix,

                //Protocol
                Protocol = Protocol.ab_eip,

                //A global timeout value that is used for Initialize/Read/Write methods
                Timeout = TimeSpan.FromMilliseconds(TIMEOUT), 
            };
            myTag.Initialize();

            //Read tag value - This pulls the value from the PLC into the local Tag value
            Console.WriteLine($"Starting tag read");
            myTag.Read();

            //Read back value from local memory
            int myDint = myTag.Value;
            Console.WriteLine($"Initial Value: {myDint}");

            //Set Tag Value
            myDint++;
            myTag.Value = myDint;

            Console.WriteLine($"Starting tag write ({myDint})");
            myTag.Write();

            //Read tag value - This pulls the value from the PLC into the local Tag value
            Console.WriteLine($"Starting synchronous tag read");
            myTag.Read();

            //Read back value from local memory
            var myDintReadBack = myTag.Value;
            Console.WriteLine($"Final Value: {myDintReadBack}");

        }
    }
}
