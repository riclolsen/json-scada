using System;
using System.Linq;
using System.Threading.Tasks;
using Xunit;

namespace libplctag.NativeImport.Tests
{
    public class UnitTest1
    {
        [Fact]
        public void Test_Dll_Load()
        {
            // The simplest way to check the Dll is working is use the decode error API which returns a string
            var output = plctag.plc_tag_decode_error(0);
            Assert.Equal("PLCTAG_STATUS_OK", output);
        }

        [Fact]
        public void Parallel_Dll_Load()
        {
            Task.WaitAll(Enumerable.Range(0, 1000000).Select(i => Task.Run(async () =>
            {
                await Task.Delay(1000);
                plctag.plc_tag_check_lib_version(0, 0, 0);
            })).ToArray());
        }
    }
}
