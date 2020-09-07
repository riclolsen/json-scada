namespace libplctag
{
    public enum PlcType
    {
        /// <summary>
        /// Control Logix-class PLC. Synonym for lgx, logix, controllogix, contrologix, compactlogix, clgx.
        /// </summary>
        ControlLogix,

        /// <summary>
        /// PLC/5 PLC. Synonym for plc5, plc.
        /// </summary>
        Plc5,

        /// <summary>
        /// SLC 500 PLC. Synonym for slc500, slc.
        /// </summary>
        Slc500,

        /// <summary>
        /// Control Logix-class PLC using the PLC/5 protocol. Synonym for lgxpccc, logixpccc, lgxplc5, lgx_pccc, logix_pccc, lgx_plc5.
        /// </summary>
        LogixPccc,

        /// <summary>
        /// Micro800-class PLC. Synonym for micrologix800, mlgx800, micro800.
        /// </summary>
        Micro800,

        /// <summary>
        /// MicroLogix PLC. Synonym for micrologix, mlgx.
        /// </summary>
        MicroLogix
    }
}