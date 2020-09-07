using System;

namespace libplctag
{

    /// <summary>
    /// An exception thrown by the underlying libplctag library
    /// </summary>
    public class LibPlcTagException : Exception
    {
        public LibPlcTagException()
        {
        }

        public LibPlcTagException(string message)
            : base(message)
        {
        }

        public LibPlcTagException(string message, Exception inner)
            : base(message, inner)
        {
        }

        public LibPlcTagException(Status status)
            : base(status.ToString())
        {
        }
    }
}
