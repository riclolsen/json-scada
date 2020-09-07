using System;
using System.Threading;
using System.Threading.Tasks;

namespace libplctag
{
    /// <summary>
    /// An interface to represent any generic tag without
    /// exposing its value
    /// </summary>
    public interface ITag : IDisposable
    {
        int[] ArrayDimensions { get; set; }
        string Gateway { get; set; }
        string Name { get; set; }
        string Path { get; set; }
        PlcType? PlcType { get; set; }
        Protocol? Protocol { get; set; }
        int? ReadCacheMillisecondDuration { get; set; }
        TimeSpan Timeout { get; set; }
        bool? UseConnectedMessaging { get; set; }

        Status GetStatus();
        void Initialize();
        Task InitializeAsync(CancellationToken token = default);
        void Read();
        Task ReadAsync(CancellationToken token = default);
        void Write();
        Task WriteAsync(CancellationToken token = default);
    }
}