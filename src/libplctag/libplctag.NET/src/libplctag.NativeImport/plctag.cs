using System;
using System.Runtime.InteropServices;

namespace libplctag.NativeImport
{
    public static class plctag
    {

        const string DLL_NAME = "plctag";


        static private bool _forceExtractLibrary = true;
        static public bool ForceExtractLibrary
        {
            get => _forceExtractLibrary;
            set
            {
                if (libraryAlreadyInitialized)
                    throw new InvalidOperationException("Library already initialized");
                _forceExtractLibrary = value;
            }
        }
        
        static private bool libraryAlreadyInitialized = false;
        static object _libraryExtractLocker = new object();
        private static void ExtractLibraryIfRequired()
        {
            // Non-blocking check
            // Except during startup, this will be hit 100% of the time
            if(!libraryAlreadyInitialized)
            {

                // Blocking check
                // This is hit if multiple threads simultaneously try to initialize the library
                lock (_libraryExtractLocker)
                {
                    if (!libraryAlreadyInitialized)
                    {
                        LibraryExtractor.Init(ForceExtractLibrary);
                        libraryAlreadyInitialized = true;
                    }
                }
            }
        }



        public static int plc_tag_check_lib_version(int req_major, int req_minor, int req_patch)
        {
            ExtractLibraryIfRequired();
            return plc_tag_check_lib_version_raw(req_major, req_minor, req_patch);
        }

        public static Int32 plc_tag_create([MarshalAs(UnmanagedType.LPStr)] string lpString, int timeout)
        {
            ExtractLibraryIfRequired();
            return plc_tag_create_raw(lpString, timeout);
        }

        public static int plc_tag_destroy(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_destroy_raw(tag);
        }

        public static int plc_tag_shutdown()
        {
            ExtractLibraryIfRequired();
            return plc_tag_shutdown_raw();
        }

        public static int plc_tag_register_callback(Int32 tag_id, callback_func func)
        {
            ExtractLibraryIfRequired();
            return plc_tag_register_callback_raw(tag_id, func);
        }

        public static int plc_tag_unregister_callback(Int32 tag_id)
        {
            ExtractLibraryIfRequired();
            return plc_tag_unregister_callback_raw(tag_id);
        }

        public static int plc_tag_register_logger(log_callback_func func)
        {
            ExtractLibraryIfRequired();
            return plc_tag_register_logger_raw(func);
        }
        public static int plc_tag_unregister_logger(Int32 tag_id)
        {
            ExtractLibraryIfRequired();
            return plc_tag_unregister_logger_raw(tag_id);
        }

        public static int plc_tag_lock(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_lock_raw(tag);
        }

        public static int plc_tag_unlock(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_unlock_raw(tag);
        }
        public static int plc_tag_status(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_status_raw(tag);
        }

        public static string plc_tag_decode_error(int err)
        {
            ExtractLibraryIfRequired();
            return Marshal.PtrToStringAnsi(plc_tag_decode_error_raw(err));
        }

        public static int plc_tag_read(Int32 tag, int timeout)
        {
            ExtractLibraryIfRequired();
            return plc_tag_read_raw(tag, timeout);
        }

        public static int plc_tag_write(Int32 tag, int timeout)
        {
            ExtractLibraryIfRequired();
            return plc_tag_write_raw(tag, timeout);
        }

        public static int plc_tag_get_size(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_size_raw(tag);
        }

        public static int plc_tag_abort(Int32 tag)
        {
            ExtractLibraryIfRequired();
            return plc_tag_abort_raw(tag);
        }

        public static int plc_tag_get_int_attribute(Int32 tag, string attrib_name, int default_value)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_int_attribute_raw(tag, attrib_name, default_value);
        }

        public static int plc_tag_set_int_attribute(Int32 tag, string attrib_name, int new_value)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_int_attribute_raw(tag, attrib_name, new_value);
        }

        public static UInt64 plc_tag_get_uint64(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_uint64_raw(tag, offset);
        }

        public static Int64 plc_tag_get_int64(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_int64_raw(tag, offset);
        }

        public static int plc_tag_set_uint64(Int32 tag, int offset, UInt64 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_uint64_raw(tag, offset, val);
        }

        public static int plc_tag_set_int64(Int32 tag, int offset, Int64 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_int64_raw(tag, offset, val);
        }

        public static double plc_tag_get_float64(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_float64_raw(tag, offset);
        }

        public static int plc_tag_set_float64(Int32 tag, int offset, double val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_float64_raw(tag, offset, val);
        }
        public static UInt32 plc_tag_get_uint32(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_uint32_raw(tag, offset);
        }

        public static Int32 plc_tag_get_int32(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_int32_raw(tag, offset);
        }

        public static int plc_tag_set_uint32(Int32 tag, int offset, UInt32 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_uint32_raw(tag, offset, val);
        }

        public static int plc_tag_set_int32(Int32 tag, int offset, Int32 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_int32_raw(tag, offset, val);
        }

        public static float plc_tag_get_float32(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_float32_raw(tag, offset);
        }

        public static int plc_tag_set_float32(Int32 tag, int offset, float val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_float32_raw(tag, offset, val);
        }

        public static UInt16 plc_tag_get_uint16(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_uint16_raw(tag, offset);
        }

        public static Int16 plc_tag_get_int16(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_int16_raw(tag, offset);
        }

        public static int plc_tag_set_uint16(Int32 tag, int offset, UInt16 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_uint16_raw(tag, offset, val);
        }

        public static int plc_tag_set_int16(Int32 tag, int offset, Int16 val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_int16_raw(tag, offset, val);
        }

        public static byte plc_tag_get_uint8(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_uint8_raw(tag, offset);
        }

        public static sbyte plc_tag_get_int8(Int32 tag, int offset)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_int8_raw(tag, offset);
        }

        public static int plc_tag_set_uint8(Int32 tag, int offset, byte val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_uint8_raw(tag, offset, val);
        }

        public static int plc_tag_set_int8(Int32 tag, int offset, sbyte val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_int8_raw(tag, offset, val);
        }

        public static int plc_tag_get_bit(Int32 tag, int offset_bit)
        {
            ExtractLibraryIfRequired();
            return plc_tag_get_bit_raw(tag, offset_bit);
        }

        public static int plc_tag_set_bit(Int32 tag, int offset_bit, int val)
        {
            ExtractLibraryIfRequired();
            return plc_tag_set_bit_raw(tag, offset_bit, val);
        }
        public static void plc_tag_set_debug_level(int debug_level)
        {
            ExtractLibraryIfRequired();
            plc_tag_set_debug_level_raw(debug_level);
        }












        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_check_lib_version), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_check_lib_version_raw(int req_major, int req_minor, int req_patch);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_create), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true, CharSet = CharSet.Ansi)]
        static extern Int32 plc_tag_create_raw([MarshalAs(UnmanagedType.LPStr)] string lpString, int timeout);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_destroy), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_destroy_raw(Int32 tag);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_shutdown), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_shutdown_raw();


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_register_callback), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_register_callback_raw(Int32 tag_id, callback_func func);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_unregister_callback), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_unregister_callback_raw(Int32 tag_id);


        [UnmanagedFunctionPointer(CallingConvention.Cdecl)]
        public delegate void callback_func(Int32 tag_id, Int32 event_id, Int32 status);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_register_logger), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_register_logger_raw(log_callback_func func);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_unregister_logger), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_unregister_logger_raw(Int32 tag_id);


        [UnmanagedFunctionPointer(CallingConvention.Cdecl, CharSet = CharSet.Ansi)]
        public delegate void log_callback_func(Int32 tag_id, int debug_level, [MarshalAs(UnmanagedType.LPStr)] string message);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_lock), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_lock_raw(Int32 tag);
        

        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_unlock), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_unlock_raw(Int32 tag);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_status), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_status_raw(Int32 tag);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_decode_error), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true, CharSet = CharSet.Ansi)]
        static extern IntPtr plc_tag_decode_error_raw(int err);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_read), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_read_raw(Int32 tag, int timeout);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_write), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_write_raw(Int32 tag, int timeout);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_size), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_get_size_raw(Int32 tag);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_abort), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_abort_raw(Int32 tag);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_int_attribute), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true, CharSet = CharSet.Ansi)]
        static extern int plc_tag_get_int_attribute_raw(Int32 tag, [MarshalAs(UnmanagedType.LPStr)] string attrib_name, int default_value);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_int_attribute), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true, CharSet = CharSet.Ansi)]
        static extern int plc_tag_set_int_attribute_raw(Int32 tag, [MarshalAs(UnmanagedType.LPStr)] string attrib_name, int new_value);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_uint64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern UInt64 plc_tag_get_uint64_raw(Int32 tag, int offset);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_int64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern Int64 plc_tag_get_int64_raw(Int32 tag, int offset);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_uint64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_uint64_raw(Int32 tag, int offset, UInt64 val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_int64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_int64_raw(Int32 tag, int offset, Int64 val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_float64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern double plc_tag_get_float64_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_float64), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_float64_raw(Int32 tag, int offset, double val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_uint32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern UInt32 plc_tag_get_uint32_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_int32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern Int32 plc_tag_get_int32_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_uint32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_uint32_raw(Int32 tag, int offset, UInt32 val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_int32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_int32_raw(Int32 tag, int offset, Int32 val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_float32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern float plc_tag_get_float32_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_float32), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_float32_raw(Int32 tag, int offset, float val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_uint16), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern UInt16 plc_tag_get_uint16_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_int16), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern Int16 plc_tag_get_int16_raw(Int32 tag, int offset);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_uint16), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_uint16_raw(Int32 tag, int offset, UInt16 val);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_int16), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_int16_raw(Int32 tag, int offset, Int16 val);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_uint8), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern byte plc_tag_get_uint8_raw(Int32 tag, int offset);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_int8), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern sbyte plc_tag_get_int8_raw(Int32 tag, int offset);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_uint8), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_uint8_raw(Int32 tag, int offset, byte val);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_int8), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_int8_raw(Int32 tag, int offset, sbyte val);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_get_bit), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_get_bit_raw(Int32 tag, int offset_bit);

        
        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_bit), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern int plc_tag_set_bit_raw(Int32 tag, int offset_bit, int val);


        [DllImport(DLL_NAME, EntryPoint = nameof(plc_tag_set_debug_level), CallingConvention = CallingConvention.Cdecl, ExactSpelling = true)]
        static extern void plc_tag_set_debug_level_raw(int debug_level);


    }
}