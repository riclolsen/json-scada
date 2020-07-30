/*
 *  Copyright 2016 MZ Automation GmbH
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

using System;

using System.Net;
using System.Net.Sockets;
using System.Text;
using System.Threading;

namespace lib60870
{
    /// <summary>
    /// Some system related helper functions
    /// </summary>
    public static class SystemUtils
    {
        private static DateTime Jan1st1970 = new DateTime(1970, 1, 1, 0, 0, 0, DateTimeKind.Utc);

        /// <summary>
        /// Get the current time in milliseconds since epoch.
        /// </summary>
        /// <returns>The current time</returns>
        public static long currentTimeMillis()
        {
            return (long)((DateTime.UtcNow - Jan1st1970).TotalMilliseconds);
        }

        /// <summary>
        /// Convert a DateTime in milliseconds time
        /// </summary>
        /// <returns>The converted time in milliseconds</returns>
        /// <param name="time">DateTime to convert</param>
        public static long ToTimeMillis(DateTime time)
        {
            return (long)((time.ToUniversalTime() - Jan1st1970).TotalMilliseconds);
        }

        /// <summary>
        /// Convert a milliseconds time to a DateTime
        /// </summary>
        /// <returns>the converted DateTime instance</returns>
        /// <param name="millis">milliseconds time to convert</param>
        public static DateTime FromMillis(long millis)
        {
            DateTime dtDateTime = Jan1st1970;
            dtDateTime = dtDateTime.AddMilliseconds(millis).ToLocalTime();
            return dtDateTime;
        }

    }
}
