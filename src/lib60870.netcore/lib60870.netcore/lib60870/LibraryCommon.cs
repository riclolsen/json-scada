using System;

namespace lib60870
{

    /// <summary>
    /// Common information functions about the library
    /// </summary>
    public class LibraryCommon
    {
        /// <summary>
        /// Library major version number
        /// </summary>
        public const int VERSION_MAJOR = 2;

        /// <summary>
        /// Library minor version number
        /// </summary>
        public const int VERSION_MINOR = 3;

        /// <summary>
        /// Library patch number
        /// </summary>
        public const int VERSION_PATCH = 0;

        /// <summary>
        /// Gets the library version as string {major}.{minor}.{patch}.
        /// </summary>
        /// <returns>The library version as string.</returns>
        public static string GetLibraryVersionString()
        {
            return "" + VERSION_MAJOR + "." + VERSION_MINOR + "." + VERSION_PATCH;
        }
    }

    /// <summary>
    /// Raw message handler. Can be used to access the raw message.
    /// Returns true when message should be handled by the protocol stack, false, otherwise.
    /// </summary>
	public delegate bool RawMessageHandler(object parameter,byte[] message,int messageSize);
}

