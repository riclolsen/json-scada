/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using IEC61850.SCL.DataModel;
using System;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclSettingControl
    {
        public XmlNode xmlNode;
        private XmlDocument xmlDocument;
        public IEDModelNode Parent;


        public SclSettingControl(SclDocument SclxmlDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            XmlAttribute nameAttr = xmlNode.Attributes["numOfSGs"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "SettingControl has no numOfSGs attribute", this, "numOfSGs");

        }

        public SclSettingControl(XmlDocument xmlDoc, string numOfSGs)
        {
            xmlDocument = xmlDoc;
            xmlNode = xmlDoc.CreateElement("SettingControl", SclDocument.SCL_XMLNS);

            XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "numOfSGs", numOfSGs);
            XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "actSG", "1");

        }

        public string Desc
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "desc");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "desc", value);
            }
        }

        public int NumOfSGs
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "numOfSGs");

                if (valStr != null)
                {
                    int retVal = -1;
                    Int32.TryParse(valStr, out retVal);

                    return (retVal);
                }
                else
                    return (-1);
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "numOfSGs", value.ToString());
            }
        }

        public int ActSG
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "actSG");

                if (valStr != null)
                {
                    int retVal = -1;
                    Int32.TryParse(valStr, out retVal);

                    return (retVal);
                }
                else
                    return (-1);
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "actSG", value.ToString());
            }

        }

        public int ResvTms
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "resvTms");

                if (valStr != null)
                {
                    int retVal = -1;
                    Int32.TryParse(valStr, out retVal);

                    return (retVal);
                }
                else
                    return (-1);
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "resvTms", value.ToString());
            }

        }

    }
}
