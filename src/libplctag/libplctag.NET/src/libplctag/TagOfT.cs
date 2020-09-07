using libplctag.DataTypes;
using System;
using System.Collections;
using System.Collections.Generic;
using System.Net;
using System.Threading;
using System.Threading.Tasks;

namespace libplctag
{
    /// <summary>
    /// A class that allows for strongly-typed objects tied to PLC tags
    /// </summary>
    /// <typeparam name="M">A mapper class that handles data conversion</typeparam>
    /// <typeparam name="T">The desired C# type of Tag.Value</typeparam>
    public class Tag<M, T> : IDisposable, ITag where M : IPlcMapper<T>, new()
    {

        private readonly Tag _tag;
        private readonly IPlcMapper<T> _plcMapper;

        public Tag()
        {
            _plcMapper = new M();
            _tag = new Tag()
            {
                ElementSize = _plcMapper.ElementSize,
            };
        }

        /// <summary>
        /// Determines the type of the PLC protocol.
        /// </summary>
        public Protocol? Protocol
        {
            get => _tag.Protocol;
            set => _tag.Protocol = value;
        }

        /// <summary>
        /// This tells the library what host name or IP address to use for the PLC 
        /// or the gateway to the PLC (in the case that the PLC is remote).
        /// </summary>
        public string Gateway
        {
            get => _tag.Gateway;
            set => _tag.Gateway = value;
        }

        /// <summary>
        /// This attribute is required for CompactLogix/ControlLogix tags 
        /// and for tags using a DH+ protocol bridge (i.e. a DHRIO module) to get to a PLC/5, SLC 500, or MicroLogix PLC on a remote DH+ link. 
        /// The attribute is ignored if it is not a DH+ bridge route, but will generate a warning if debugging is active. 
        /// Note that Micro800 connections must not have a path attribute.
        /// </summary>
        public string Path
        {
            get => _tag.Path;
            set => _tag.Path = value;
        }

        /// <summary>
        /// The type of PLC
        /// </summary>
        public PlcType? PlcType
        {
            get => _tag.PlcType;
            set => _tag.PlcType = value;
        }

        /// <summary>
        /// Dimensions of Value if it is an array
        /// Ex. {2, 10} for a 2 column, 10 row array
        /// Non-arrays can use null (default)
        /// </summary>
        public int[] ArrayDimensions
        {
            get => _plcMapper.ArrayDimensions;
            set
            {
                _plcMapper.ArrayDimensions = value;
                _tag.ElementCount = _plcMapper.GetElementCount();
            }
        }

        /// <summary>
        /// This is the full name of the tag.
        /// For program tags, prepend `Program:{ProgramName}.` 
        /// where {ProgramName} is the name of the program in which the tag is created.
        /// </summary>
        public string Name
        {
            get => _tag.Name;
            set => _tag.Name = value;
        }

        /// <summary>
        /// Control whether to use connected or unconnected messaging. 
        /// Only valid on Logix-class PLCs. Connected messaging is required on Micro800 and DH+ bridged links. 
        /// Default is PLC-specific and link-type specific. Generally you do not need to set this.
        /// </summary>
        public bool? UseConnectedMessaging
        {
            get => _tag.UseConnectedMessaging;
            set => _tag.UseConnectedMessaging = value;
        }

        /// <summary>
        /// Use this attribute to cause the tag read operations to cache data the requested number of milliseconds. 
        /// This can be used to lower the actual number of requests against the PLC. 
        /// Example read_cache_ms=100 will result in read operations no more often than once every 100 milliseconds.
        /// </summary>
        public int? ReadCacheMillisecondDuration
        {
            get => _tag.ReadCacheMillisecondDuration;
            set => _tag.ReadCacheMillisecondDuration = value;
        }

        /// <summary>
        /// A global timeout value that is used for Initialize/Read/Write methods.
        /// It applies to both synchronous and async calls.
        /// </summary>
        public TimeSpan Timeout
        {
            get => _tag.Timeout;
            set => _tag.Timeout = value;
        }

        /// <summary>
        /// Creates the underlying data structures and references required before tag operations.
        /// </summary>
        public void Initialize()
        {
            _tag.Initialize();
            DecodeAll();
        }

        /// <summary>
        /// Creates the underlying data structures and references required before tag operations.
        /// </summary>
        public async Task InitializeAsync(CancellationToken token = default)
        {
            await _tag.InitializeAsync(token);
            DecodeAll();
        }

        /// <summary>
        /// Reading a tag brings the data at the time of read into the local memory of the PC running the library. 
        /// The data is not automatically kept up to date. 
        /// If you need to find out the data periodically, you need to read the tag periodically.
        /// </summary>
        /// <param name="token">Optional Cancellation Token</param>
        /// <returns>Task</returns>
        public async Task ReadAsync(CancellationToken token = default)
        {
            if (!_tag.IsInitialized)
                await _tag.InitializeAsync(token);

            await _tag.ReadAsync(token);
            DecodeAll();
        }

        /// <summary>
        /// Reading a tag brings the data at the time of read into the local memory of the PC running the library. 
        /// The data is not automatically kept up to date. 
        /// If you need to find out the data periodically, you need to read the tag periodically.
        /// </summary>
        public void Read()
        {
            if (!_tag.IsInitialized)
                _tag.Initialize();

            _tag.Read();
            DecodeAll();
        }

        /// <summary>
        /// Writing a tag sends the data from Value (local memory) to the target PLC.
        /// </summary>
        /// <param name="token">Optional Cancellation Token</param>
        /// <returns>Task</returns>
        public async Task WriteAsync(CancellationToken token = default)
        {
            if (!_tag.IsInitialized)
                await _tag.InitializeAsync(token);

            EncodeAll();
            await _tag.WriteAsync(token);
        }

        /// <summary>
        /// Writing a tag sends the data from Value (local memory) to the target PLC.
        /// </summary>
        public void Write()
        {
            if (!_tag.IsInitialized)
                _tag.Initialize();

            EncodeAll();
            _tag.Write();
        }


        void DecodeAll()
        {
            Value = _plcMapper.Decode(_tag);
        }

        void EncodeAll()
        {
            _plcMapper.Encode(_tag, Value);
        }

        /// <summary>
        /// Check the operational status of the tag
        /// </summary>
        /// <returns>Tag's current status</returns>
        public Status GetStatus() => _tag.GetStatus();

        public void Dispose() => _tag.Dispose();

        ~Tag()
        {
            Dispose();
        }

        /// <summary>
        /// The local memory value that can be transferred to/from the PLC
        /// </summary>
        public T Value { get; set; }

    }
}
