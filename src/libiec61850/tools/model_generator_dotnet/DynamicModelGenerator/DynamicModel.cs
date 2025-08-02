using IEC61850.SCL;
using IEC61850.SCL.DataModel;
using System;
using System.IO;
using System.Linq;

namespace DynamicModel
{
    public class DynamicModel
    {
        public DynamicModel(string fileName, FileStream stream,
            string iedName, string accessPointName)
        {
            try
            {
                SclDocument sclParser = new SclDocument(fileName);

                SclIED ied = null;

                if (iedName == null)
                    ied = sclParser.IEDs.First();
                else
                    ied = sclParser.IEDs.Find(x => x.Name == iedName);

                if (ied == null)
                {
                    throw new Exception("IED model not found in SCL file! Exit.");
                }

                SclAccessPoint accessPoint = null;

                if (accessPointName == null)
                    accessPoint = ied.AccessPoints.First();
                else
                    accessPoint = ied.AccessPoints.Find(x => x.Name == accessPointName);

                if (accessPoint == null)
                {
                    throw new Exception("AccessPoint not found in SCL file! Exit.");
                }

                IEDDataModel iedModel = sclParser.GetDataModel(ied.Name, accessPoint.Name);


                using (StreamWriter writer = new StreamWriter(stream))
                {

                    DynamicModelGenerator dynamicModelGenerator = new DynamicModelGenerator(sclParser, writer, iedModel, accessPoint);
                }
            }
            catch (Exception ex)
            {
                Console.Write(ex.ToString());
            }

        }
    }
}
