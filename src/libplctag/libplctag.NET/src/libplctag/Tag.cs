using System;
using System.Collections.Generic;
using System.Linq;
using System.Net;
using System.Threading;
using System.Threading.Tasks;
using libplctag.NativeImport;

namespace libplctag
{

    public sealed class Tag : IDisposable
    {

        private const int ASYNC_STATUS_POLL_INTERVAL = 2;
        private static readonly TimeSpan defaultTimeout = TimeSpan.FromSeconds(100);
        private static readonly TimeSpan maxTimeout = TimeSpan.FromMilliseconds(int.MaxValue);

        private string _name;
        public string Name
        {
            get => _name;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _name = value;
            }
        }

        private Protocol? _protocol;
        public Protocol? Protocol
        {
            get => _protocol;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _protocol = value;
            }
        }

        private string _gateway;
        public string Gateway
        {
            get => _gateway;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _gateway = value;
            }
        }

        private PlcType? _plcType;
        public PlcType? PlcType
        {
            get => _plcType;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _plcType = value;
            }
        }

        private string _path;
        public string Path
        {
            get => _path;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _path = value;
            }
        }

        private int? _elementSize;
        public int? ElementSize
        {
            get => _elementSize;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _elementSize = value;
            }
        }

        private int? _elementCount;
        public int? ElementCount
        {
            get => _elementCount;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _elementCount = value;
            }
        }

        private bool? _useConnectedMessaging;
        public bool? UseConnectedMessaging
        {
            get => _useConnectedMessaging;
            set
            {
                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");
                _useConnectedMessaging = value;
            }
        }

        private int? _readCacheMillisecondDuration;
        public int? ReadCacheMillisecondDuration
        {
            get
            {

                if (!IsInitialized)
                    return _readCacheMillisecondDuration;

                return GetIntAttribute("read_cache_ms");
            }
            set
            {
                if (!IsInitialized)
                {
                    _readCacheMillisecondDuration = value;
                    return;
                }

                SetIntAttribute("read_cache_ms", value.Value);
            }
        }

        private TimeSpan timeout = defaultTimeout;
        public TimeSpan Timeout
        {
            get { return timeout; }
            set
            {
                if (value <= TimeSpan.Zero || value > maxTimeout)
                {
                    throw new ArgumentOutOfRangeException("value");
                }
                timeout = value;
            }
        }

        private int tagHandle;

        ~Tag()
        {
            Dispose();
        }

        private bool _isDisposed = false;
        public void Dispose()
        {
            if (_isDisposed)
                return;

            if (!IsInitialized)
                return;

            var result = (Status)plctag.plc_tag_destroy(tagHandle);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);

            _isDisposed = true;
        }

        public void Abort()
        {
            var result = (Status)plctag.plc_tag_abort(tagHandle);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        private string GetAttributeString()
        {
            var attributes = new Dictionary<string, string>();

            attributes.Add("protocol", this.Protocol.ToString());
            attributes.Add("gateway", this.Gateway);
            attributes.Add("path", Path);
            attributes.Add("plc", PlcType.ToString().ToLower());
            attributes.Add("elem_size", ElementSize?.ToString());
            attributes.Add("elem_count", ElementCount?.ToString());
            attributes.Add("name", Name);
            attributes.Add("read_cache_ms", ReadCacheMillisecondDuration?.ToString());
            if (UseConnectedMessaging.HasValue)
                attributes.Add("use_connected_msg", UseConnectedMessaging.Value ? "1" : "0");

            string separator = "&";
            return string.Join(separator, attributes.Where(attr => attr.Value != null).Select(attr => $"{attr.Key}={attr.Value}"));

        }

        public bool IsInitialized { get; private set; }

        /// <summary>
        /// Initializes the tag by establishing necessary connections.
        /// Can only be called once per instance.
        /// Timeout is controlled via class property.
        /// </summary>
        public void Initialize()
        {
            var millisecondTimeout = (int)Timeout.TotalMilliseconds;

            if (IsInitialized)
                throw new InvalidOperationException("Already initialized");

            if (millisecondTimeout <= 0)
                throw new ArgumentOutOfRangeException(nameof(millisecondTimeout), "Must be greater than 0 for a synchronous initialization");

            var attributeString = GetAttributeString();

            var result = plctag.plc_tag_create(attributeString, millisecondTimeout);
            if (result < 0)
                throw new LibPlcTagException((Status)result);
            else
                tagHandle = result;

            IsInitialized = true;
        }

        /// <summary>
        /// Initializes the tag by establishing necessary connections.
        /// Can only be called once per instance.
        /// Timeout is controlled via class property.
        /// </summary>
        public async Task InitializeAsync(CancellationToken token = default)
        {
            using (var cts = CancellationTokenSource.CreateLinkedTokenSource(token))
            {
                cts.CancelAfter(Timeout);

                if (IsInitialized)
                    throw new InvalidOperationException("Already initialized");

                var attributeString = GetAttributeString();

                var result = plctag.plc_tag_create(attributeString, 0);
                if (result < 0)
                    throw new LibPlcTagException((Status)result);
                else
                    tagHandle = result;

                using (token.Register(() => Abort()))
                {
                    while (GetStatus() == Status.Pending)
                    {
                        await Task.Delay(ASYNC_STATUS_POLL_INTERVAL, token);
                    }
                }

                var status = GetStatus();
                if (status != Status.Ok)
                    throw new LibPlcTagException(status);

                IsInitialized = true;
            }
        }

        /// <summary>
        /// Executes a synchronous read on a tag.
        /// Timeout is controlled via class property.
        /// </summary>
        public void Read()
        {
            var millisecondTimeout = (int)Timeout.TotalMilliseconds;

            var result = (Status)plctag.plc_tag_read(tagHandle, millisecondTimeout);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        /// <summary>
        /// Executes an asynch read on a tag.
        /// Timeout is controlled via class property.
        /// </summary>
        public async Task ReadAsync(CancellationToken token = default)
        {
            using (var cts = CancellationTokenSource.CreateLinkedTokenSource(token))
            {
                cts.CancelAfter(Timeout);
                var status = (Status)plctag.plc_tag_read(tagHandle, 0);

                using (token.Register(() => Abort()))
                {
                    while (status == Status.Pending)
                    {
                        await Task.Delay(ASYNC_STATUS_POLL_INTERVAL, token);
                        status = GetStatus();
                    }
                }

                if (status != Status.Ok)
                    throw new LibPlcTagException(status);
            }
        }

        /// <summary>
        /// Executes a synchronous write on a tag.
        /// Timeout is controlled via class property.
        /// </summary>
        public void Write()
        {
            var millisecondTimeout = (int)Timeout.TotalMilliseconds;

            var result = (Status)plctag.plc_tag_write(tagHandle, millisecondTimeout);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);

        }

        /// <summary>
        /// Executes an asynch write on a tag.
        /// Timeout is controlled via class property.
        /// </summary>
        public async Task WriteAsync(CancellationToken token = default)
        {
            using (var cts = CancellationTokenSource.CreateLinkedTokenSource(token))
            {
                cts.CancelAfter(Timeout);
                var status = (Status)plctag.plc_tag_write(tagHandle, 0);

                using (token.Register(() => Abort()))
                {
                    while (status == Status.Pending)
                    {
                        await Task.Delay(ASYNC_STATUS_POLL_INTERVAL, token);
                        status = GetStatus();
                    }
                }

                if (status != Status.Ok)
                    throw new LibPlcTagException(status);
            }
        }

        public int GetSize()
        {
            var result = plctag.plc_tag_get_size(tagHandle);
            if (result < 0)
                throw new LibPlcTagException((Status)result);
            else
                return result;
        }

        public Status GetStatus() => (Status)plctag.plc_tag_status(tagHandle);

        public byte[] GetBuffer()
        {
            var tagSize = GetSize();
            var temp = new byte[tagSize];

            for (int ii = 0; ii < tagSize; ii++)
                temp[ii] = GetUInt8(ii);

            return temp;
        }


        private int GetIntAttribute(string attributeName)
        {
            var result = plctag.plc_tag_get_int_attribute(tagHandle, attributeName, int.MinValue);
            if (result == int.MinValue)
                throw new LibPlcTagException();
            return result;
        }

        private void SetIntAttribute(string attributeName, int value)
        {
            var result = (Status)plctag.plc_tag_set_int_attribute(tagHandle, attributeName, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public bool GetBit(int offset)
        {
            var result = plctag.plc_tag_get_bit(tagHandle, offset);
            if (result == 0)
                return false;
            else if (result == 1)
                return true;
            else
                throw new LibPlcTagException((Status)result);
        }

        public void SetBit(int offset, bool value)
        {
            int valueAsInteger = value == true ? 1 : 0;
            var result = (Status)plctag.plc_tag_set_bit(tagHandle, offset, valueAsInteger);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public ulong GetUInt64(int offset)
        {
            var result = plctag.plc_tag_get_uint64(tagHandle, offset);
            if (result == ulong.MaxValue)
                throw new LibPlcTagException();
            return result;
        }
        public void SetUInt64(int offset, ulong value)
        {
            var result = (Status)plctag.plc_tag_set_uint64(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public long GetInt64(int offset)
        {
            var result = plctag.plc_tag_get_int64(tagHandle, offset);
            if (result == long.MinValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetInt64(int offset, long value)
        {
            var result = (Status)plctag.plc_tag_set_int64(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public uint GetUInt32(int offset)
        {
            var result = plctag.plc_tag_get_uint32(tagHandle, offset);
            if (result == uint.MaxValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetUInt32(int offset, uint value)
        {
            var result = (Status)plctag.plc_tag_set_uint32(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public int GetInt32(int offset)
        {
            var result = plctag.plc_tag_get_int32(tagHandle, offset);
            if (result == int.MinValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetInt32(int offset, int value)
        {
            var result = (Status)plctag.plc_tag_set_int32(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public ushort GetUInt16(int offset)
        {
            var result = plctag.plc_tag_get_uint16(tagHandle, offset);
            if (result == ushort.MaxValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetUInt16(int offset, ushort value)
        {
            var result = (Status)plctag.plc_tag_set_uint16(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public short GetInt16(int offset)
        {
            var result = plctag.plc_tag_get_int16(tagHandle, offset);
            if (result == short.MinValue)
                throw new LibPlcTagException();
            return result;
        }
        public void SetInt16(int offset, short value)
        {
            var result = (Status)plctag.plc_tag_set_int16(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public byte GetUInt8(int offset)
        {
            var result = plctag.plc_tag_get_uint8(tagHandle, offset);
            if (result == byte.MaxValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetUInt8(int offset, byte value)
        {
            var result = (Status)plctag.plc_tag_set_uint8(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public sbyte GetInt8(int offset)
        {
            var result = plctag.plc_tag_get_int8(tagHandle, offset);
            if (result == sbyte.MinValue)
                throw new LibPlcTagException();
            return result;
        }

        public void SetInt8(int offset, sbyte value)
        {
            var result = (Status)plctag.plc_tag_set_int8(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public double GetFloat64(int offset)
        {
            var result = plctag.plc_tag_get_float64(tagHandle, offset);
            if (result == double.MinValue)
                throw new LibPlcTagException();
            return result;
        }
        public void SetFloat64(int offset, double value)
        {
            var result = (Status)plctag.plc_tag_set_float64(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

        public float GetFloat32(int offset)
        {
            var result = plctag.plc_tag_get_float32(tagHandle, offset);
            if (result == float.MinValue)
                throw new LibPlcTagException();
            return result;
        }
        public void SetFloat32(int offset, float value)
        {
            var result = (Status)plctag.plc_tag_set_float32(tagHandle, offset, value);
            if (result != Status.Ok)
                throw new LibPlcTagException(result);
        }

    }

}