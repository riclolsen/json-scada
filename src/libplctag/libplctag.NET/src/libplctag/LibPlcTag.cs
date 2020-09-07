using libplctag.NativeImport;
using System;

namespace libplctag
{
    /// <summary>
    /// A static class used to access some additional features of the libplctag base library
    /// </summary>
    public static class LibPlcTag
    {
        private const int LIB_ATTRIBUTE_POINTER = 0;

        static public int VersionMajor => plctag.plc_tag_get_int_attribute(LIB_ATTRIBUTE_POINTER, "version_major", int.MinValue);
        static public int VersionMinor => plctag.plc_tag_get_int_attribute(LIB_ATTRIBUTE_POINTER, "version_minor", int.MinValue);
        static public int VersionPatch => plctag.plc_tag_get_int_attribute(LIB_ATTRIBUTE_POINTER, "version_patch", int.MinValue);

        /// <summary>
        /// Check if base library meets version requirements
        /// </summary>
        /// <param name="requiredMajor">Major</param>
        /// <param name="requiredMinor">Minor</param>
        /// <param name="requiredPatch">Patch</param>
        /// <returns></returns>
        static public bool IsRequiredVersion(int requiredMajor, int requiredMinor, int requiredPatch)
        {
            var result = (Status)plctag.plc_tag_check_lib_version(requiredMajor, requiredMinor, requiredPatch);

            if (result == Status.Ok)
                return true;
            else if (result == Status.ErrorUnsupported)
                return false;
            else
                throw new NotImplementedException();
        }

        /// <summary>
        /// Sets a debug level for the underlying libplctag library
        /// </summary>
        static public DebugLevel DebugLevel
        {
            get => (DebugLevel)plctag.plc_tag_get_int_attribute(LIB_ATTRIBUTE_POINTER, "debug", int.MinValue);
            set => plctag.plc_tag_set_debug_level((int)value);
        }

    }
}
