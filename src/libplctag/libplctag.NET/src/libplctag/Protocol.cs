namespace libplctag
{
    public enum Protocol
    {
        /// <summary>
        /// Allen-Bradley specific flavor of EIP
        /// </summary>
        ab_eip,

        /// <summary>
        /// A Modbus TCP implementation used by many PLCs
        /// </summary>
        modbus_tcp
    }
}