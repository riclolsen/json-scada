/*
 *  AcseAuthenticationParameter.cs
 *
 *  Copyright 2025-2025 Michael Zillgith
 *
 *  This file is part of libIEC61850.
 *
 *  libIEC61850 is free software: you can redistribute it and/or modify
 *  it under the terms of the GNU General Public License as published by
 *  the Free Software Foundation, either version 3 of the License, or
 *  (at your option) any later version.
 *
 *  libIEC61850 is distributed in the hope that it will be useful,
 *  but WITHOUT ANY WARRANTY; without even the implied warranty of
 *  MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE.  See the
 *  GNU General Public License for more details.
 *
 *  You should have received a copy of the GNU General Public License
 *  along with libIEC61850.  If not, see <http://www.gnu.org/licenses/>.
 *
 *  See COPYING file for the complete license text.
 */
using System;
using System.Runtime.InteropServices;
using System.Text;

// IEC 61850 API for the libiec61850 .NET wrapper library
namespace IEC61850
{
    /// <summary>
    /// Authentication mechanism used by AcseAuthenticator
    /// </summary>
    public enum AcseAuthenticationMechanism
    {
        /** Neither ACSE nor TLS authentication used */
        ACSE_AUTH_NONE = 0,

        /** Use ACSE password for client authentication */
        ACSE_AUTH_PASSWORD = 1,

        /** Use ACSE certificate for client authentication */
        ACSE_AUTH_CERTIFICATE = 2,

        /** Use TLS certificate for client authentication */
        ACSE_AUTH_TLS = 3
    }


    public class AcseAuthenticationParameter
    {
        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr AcseAuthenticationParameter_create();

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern void AcseAuthenticationParameter_setAuthMechanism(IntPtr self, int mechanism);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern void AcseAuthenticationParameter_setPassword(IntPtr self, string password);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern int AcseAuthenticationParameter_getAuthMechanism(IntPtr self);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr AcseAuthenticationParameter_getPassword(IntPtr self);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern int AcseAuthenticationParameter_getPasswordLength(IntPtr self);

        private IntPtr self = IntPtr.Zero;

        public AcseAuthenticationParameter()
        {
            self = AcseAuthenticationParameter_create();
        }

        public AcseAuthenticationParameter(IntPtr self)
        {
            this.self = self;
        }

        public void SetAuthMechanism(AcseAuthenticationMechanism acseAuthenticationMechanism)
        {
            AcseAuthenticationParameter_setAuthMechanism(self, (int)acseAuthenticationMechanism);
        }

        public void SetPassword(string password)
        {
            AcseAuthenticationParameter_setPassword(self, password);
        }

        public AcseAuthenticationMechanism GetAuthMechanism()
        {
            return (AcseAuthenticationMechanism)AcseAuthenticationParameter_getAuthMechanism(self);
        }

        public string GetPasswordString()
        {
            try
            {
                byte[] password = GetPasswordByteArray();

                return Encoding.UTF8.GetString(password);
            }
            catch (Exception)
            {
                return null;
            }
        }

        public byte[] GetPasswordByteArray()
        {
            IntPtr password = AcseAuthenticationParameter_getPassword(self);

            if (password != IntPtr.Zero)
            {
                int lenght = GetPasswordLenght();
                byte[] result = new byte[lenght];

                Marshal.Copy(password, result, 0, lenght);

                return result;

            }
            else
                return null;
        }


        public int GetPasswordLenght()
        {
            return AcseAuthenticationParameter_getPasswordLength(self);
        }
    }

    public class IsoApplicationReference
    {
        private IntPtr self = IntPtr.Zero;

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern int IsoApplicationReference_getAeQualifier(IntPtr self);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr IsoApplicationReference_getApTitle(IntPtr self);

        public IsoApplicationReference(IntPtr self)
        {
            this.self = self;
        }

        public int GetAeQualifier()
        {
            return IsoApplicationReference_getAeQualifier(self);
        }

        public ItuObjectIdentifier GetApTitle()
        {
            IntPtr identfier = IsoApplicationReference_getApTitle(self);

            if (identfier == IntPtr.Zero)
                return null;

            return new ItuObjectIdentifier(identfier);
        }

    }

    public class ItuObjectIdentifier
    {
        private IntPtr self = IntPtr.Zero;

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern int ItuObjectIdentifier_getArcCount(IntPtr self);

        [DllImport("iec61850", CallingConvention = CallingConvention.Cdecl)]
        static extern IntPtr ItuObjectIdentifier_getArc(IntPtr self);

        public ItuObjectIdentifier(IntPtr self)
        {
            this.self = self;
        }

        public int GetArcCount()
        {
            return ItuObjectIdentifier_getArcCount(self);
        }

        public ushort[] GetArcs()
        {
            int count = ItuObjectIdentifier_getArcCount(self);
            if (count <= 0 || count > 10) return Array.Empty<ushort>();

            IntPtr arcPtr = ItuObjectIdentifier_getArc(self);

            ushort[] arcs = new ushort[count];

            short[] temp = new short[count];
            Marshal.Copy(arcPtr, temp, 0, count);

            for (int i = 0; i < count; i++)
                arcs[i] = (ushort)temp[i];

            return arcs;
        }
    }
}
