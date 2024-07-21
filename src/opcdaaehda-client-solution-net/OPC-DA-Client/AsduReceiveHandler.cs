/* 
 * IEC 60870-5-104 Server Protocol driver for {json:scada}
 * {json:scada} - Copyright (c) 2024 - Ricardo L. Olsen
 * This file is part of the JSON-SCADA distribution (https://github.com/riclolsen/json-scada).
 * 
 * This program is free software: you can redistribute it and/or modify  
 * it under the terms of the GNU General Public License as published by  
 * the Free Software Foundation, version 3.
 *
 * This program is distributed in the hope that it will be useful, but 
 * WITHOUT ANY WARRANTY; without even the implied warranty of 
 * MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the GNU 
 * General Public License for more details.
 *
 * You should have received a copy of the GNU General Public License 
 * along with this program. If not, see <http://www.gnu.org/licenses/>.
 */

using System;
using System.Globalization;
using Technosoftware.DaAeHdaClient.Da;

namespace OPCDAClientDriver
{
    partial class MainClass
    {
        /// <summary>
        /// A delegate to receive data change updates from the server.
        /// </summary>
        /// <param name="subscriptionHandle">
        /// A unique identifier for the subscription assigned by the client. If the parameter
        ///	<see cref="TsCDaSubscriptionState.ClientHandle">ClientHandle</see> is not defined this
        ///	parameter is empty.</param>
        /// <param name="requestHandle">
        ///	An identifier for the request assigned by the caller. This parameter is empty if
        ///	the	corresponding parameter	in the calls Read(), Write() or Refresh() is not	defined.
        ///	Can	be used	to Cancel an outstanding operation.
        ///	</param>
        /// <param name="values">
        ///	<para class="MsoBodyText" style="MARGIN: 1pt 0in">The set of changed values.</para>
        ///	<para class="MsoBodyText" style="MARGIN: 1pt 0in">Each value will always have
        ///	item’s ClientHandle field specified.</para>
        /// </param>
        public static void OnDataChangeEvent(object subscriptionHandle, object requestHandle, TsCDaItemValueResult[] values)
        {
            Log(string.Format("----------------------------------------------------------- DataChange: {0}", values.Length));
            if (requestHandle != null)
            {
                Log("DataChange() for requestHandle :" + requestHandle.GetHashCode().ToString());
            }
            for (var i = 0; i < values.Length; i++)
            {

                string strHandle = string.Format("{0}", values[i].ClientHandle);
                // Console.Write("    Client Handle : "); Console.WriteLine(values[i].ClientHandle);
                if (values[i].Result.IsSuccess() && values[i].Value != null)
                {
                    if (values[i].Value.GetType().IsArray)
                    {
                        for (var j = 0; j < values.Length; j++)
                        {
                            Log($"{MapHandlerToConnName[strHandle]} - Change: {MapHandlerToItemName[strHandle]}  Val[{j}]: " + string.Format("{0}", values[j].Value));
                        }
                        Log($"{MapHandlerToConnName[strHandle]} - Change: {MapHandlerToItemName[strHandle]} TS: " + values[i].Timestamp.ToString(CultureInfo.InvariantCulture));
                        Log($"{MapHandlerToConnName[strHandle]} - Change: {MapHandlerToItemName[strHandle]} Q: " + values[i].Quality);
                    }
                    else
                    {
                        var valueResult = values[i];
                        //var quality = new TsCDaQuality(193);
                        //valueResult.Quality = quality;
                        //var message =
                        //    $"\r\n    Quality: is not good : {valueResult.Quality} Code:{valueResult.Quality.GetCode()} LimitBits: {valueResult.Quality.LimitBits} QualityBits: {valueResult.Quality.QualityBits} VendorBits: {valueResult.Quality.VendorBits}";
                        //if (valueResult.Quality.QualityBits != TsDaQualityBits.Good && valueResult.Quality.QualityBits != TsDaQualityBits.GoodLocalOverride)
                        //{
                        //    Log(message);
                        //}

                        Log($"{MapHandlerToConnName[strHandle]} - Change: {MapHandlerToItemName[strHandle]} Val: {values[i].Value} Q: {values[i].Quality} TS: {values[i].Timestamp.ToString(CultureInfo.InvariantCulture)}");
                    }
                }
                //Console.Write("    Result        : "); Console.WriteLine(values[i].Result.Description());
            }
            Log("----------------------------------------------------------- End");
        }
    }
}