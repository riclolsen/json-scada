using libplctag;
using libplctag.DataTypes;
using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Linq;
using System.Net;
using System.Threading;
using System.Threading.Tasks;

namespace CSharpDotNetCore
{
    class ExampleAsync
    {
        public static async Task Run()
        {
            var myTag = new Tag<DintPlcMapper, int>()
            {
                Name = "PROGRAM:SomeProgram.SomeDINT",
                Gateway = "10.10.10.10",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip,
                Timeout = TimeSpan.FromMilliseconds(1000),
            };

            await myTag.InitializeAsync();

            myTag.Value = 3737;

            await myTag.WriteAsync();

            await myTag.ReadAsync();

            int myDint = myTag.Value;

            Console.WriteLine(myDint);
        }


        public static async void SyncAsyncComparison()
        {

            Console.WriteLine("This method measures the speed of synchronous vs asynchronous reads");

            List<Tag<DintPlcMapper, int>> myTags;

            for (int ii = 0; ii < 10; ii++)
            {
                myTags = Enumerable.Range(0, 10)
                .Select(i => {
                    var myTag = new Tag<DintPlcMapper, int>()
                    {
                        Name = $"MY_DINT_ARRAY_1000[{i}]",
                        Gateway = "10.10.10.10",
                        Path = "1,0",
                        PlcType = PlcType.ControlLogix,
                        Protocol = Protocol.ab_eip,
                    };
                    myTag.Initialize();
                    return myTag;
                    })
                .ToList();

                int repetitions = 100;

                //Console.Write($"Running {repetitions} Read() calls...");
                //var syncStopWatch = Stopwatch.StartNew();
                //for (int ii = 0; ii < repetitions; ii++)
                //{
                //    // We know that it takes less than 1000ms per read, so it will return as soon as it is finished
                //    myTag.Read(1000);
                //}
                //syncStopWatch.Stop();
                //Console.WriteLine($"\ttook {(float)syncStopWatch.ElapsedMilliseconds / (float)repetitions}ms on average");


                Console.Write($"Running {repetitions} ReadAsync() calls...");
                var asyncStopWatch = Stopwatch.StartNew();
                for (int jj = 0; jj < repetitions; jj++)
                {
                    Task.WaitAll(
                        myTags[0].ReadAsync(),
                        myTags[1].ReadAsync(),
                        myTags[2].ReadAsync(),
                        myTags[3].ReadAsync(),
                        myTags[4].ReadAsync(),
                        myTags[5].ReadAsync(),
                        myTags[6].ReadAsync(),
                        myTags[7].ReadAsync(),
                        myTags[8].ReadAsync(),
                        myTags[9].ReadAsync()
                        );
                    //await myTag.ReadAsync();
                }
                asyncStopWatch.Stop();
                Console.WriteLine($"\ttook {(float)asyncStopWatch.ElapsedMilliseconds / (float)repetitions}ms on average");
            }


        }

        public static void ParallelBlockingReads()
        {

            Console.WriteLine("This method measures the speed of synchronous vs asynchronous reads");
            var myTag = new Tag<DintPlcMapper, int>()
            {
                Name = "PROGRAM:SomeProgram.SomeDINT",
                Gateway = "10.10.10.10",
                Path = "1,0",
                PlcType = PlcType.ControlLogix,
                Protocol = Protocol.ab_eip,
            };
            myTag.Initialize();

            int repetitions = 100;

            Console.Write($"Running {repetitions} calls...");
            var sw = Stopwatch.StartNew();
            for (int ii = 0; ii < repetitions; ii++)
            {
                Task.WaitAll(
                    Task.Run(() => myTag.Read()),
                    Task.Run(() => myTag.Read())
                    );
            }
            sw.Stop();

            Console.WriteLine($"\ttook {(float)sw.ElapsedMilliseconds / (float)repetitions}ms on average");

        }

        public static void SyncAsyncMultipleTagComparison(int repetitions = 1000)
        {
            Console.WriteLine("This method measures the speed of synchronous vs asynchronous reads for multiple tags simultaneously");

            SyncAsyncMultipleTagComparisonSingleRun(1, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(2, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(3, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(4, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(5, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(6, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(7, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(8, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(9, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(10, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(11, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(12, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(13, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(14, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(15, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(16, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(17, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(18, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(19, repetitions);
            SyncAsyncMultipleTagComparisonSingleRun(20, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(25, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(30, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(35, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(40, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(45, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(50, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(60, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(70, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(80, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(90, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(100, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(200, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(300, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(400, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(500, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(600, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(700, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(800, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(900, repetitions);
            //SyncAsyncMultipleTagComparisonSingleRun(1000, repetitions);
        }

        private static void SyncAsyncMultipleTagComparisonSingleRun(int maxTags, int repetitions = 10)
        {

            Console.Write($"Running {repetitions} ReadAsync() calls on {maxTags} tags simultaneously...");

            var myTags = Enumerable.Range(0, maxTags)
                .Select(i => {
                    var myTag = new Tag<DintPlcMapper, int>()
                    {
                        Name = "PROGRAM:SomeProgram.SomeDINT",
                        Gateway = "10.10.10.10",
                        Path = "1,0",
                        PlcType = PlcType.ControlLogix,
                        Protocol = Protocol.ab_eip,
                        Timeout = TimeSpan.FromMilliseconds(1000),
                    };
                    myTag.Initialize();
                    return myTag; 
                })
                .ToList();

            var asyncStopWatch = Stopwatch.StartNew();

            Task.WaitAll(myTags.Select(tag =>
            {
                return Task.Run(async () =>
                {
                    for (int ii = 0; ii < repetitions; ii++)
                    {
                        await tag.ReadAsync();
                    }
                });
            }).ToArray());

            asyncStopWatch.Stop();
            Console.WriteLine($"\ttook {(float)asyncStopWatch.ElapsedMilliseconds / (float)repetitions}ms on average");

            foreach (var tag in myTags)
            {
                tag.Dispose();
            }

        }


        public static void AsyncParallelCancellation(int maxTags = 20, int repetitions = 100)
        {

            var myTags = Enumerable.Range(0, maxTags)
                .Select(i => {
                    var myTag = new Tag<DintPlcMapper, int>()
                    {
                        Name = $"MY_DINT_ARRAY_1000[{i}]",
                        Gateway = "10.10.10.10",
                        Path = "1,0",
                        PlcType = PlcType.ControlLogix,
                        Protocol = Protocol.ab_eip,
                        Timeout = TimeSpan.FromMilliseconds(1000),
                    };
                    myTag.Initialize();
                    return myTag;
                })
                .ToList();


            int cancelAfterSeconds = 5;
            Console.WriteLine($"Starting parallel reads of {maxTags} tags. Will cancel in {cancelAfterSeconds} seconds");

            var cts = new CancellationTokenSource();
            cts.CancelAfter(cancelAfterSeconds * 1000);

            Task.WaitAll(myTags.Select(tag =>
            {
                return Task.Run(async () =>
                {
                    try
                    {
                        for (int ii = 0; ii < repetitions; ii++)
                        {
                            await tag.ReadAsync(cts.Token);
                        }
                    }
                    catch (OperationCanceledException e)
                    {
                        Console.WriteLine("Successfully Cancelled");
                    }
                });
            }).ToArray());

        }
    }
}
