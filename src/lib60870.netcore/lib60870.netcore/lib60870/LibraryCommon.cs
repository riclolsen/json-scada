/*
 *  Copyright 2016-2025 Michael Zillgith
 *
 *  This file is part of lib60870.NET
 *
 *  lib60870.NET is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  lib60870.NET is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with lib60870.NET.  If not, see <http://www.gnu.org/licenses/>.
 *
 *  See COPYING file for the complete license text.
 */

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
    public delegate bool RawMessageHandler(object parameter, byte[] message, int messageSize);
}

