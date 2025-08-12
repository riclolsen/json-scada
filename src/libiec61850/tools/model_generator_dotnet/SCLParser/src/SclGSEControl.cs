/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclGSEControl
    {
        public XmlNode xmlNode;
        private XmlDocument xmlDocument;

        public SclGSEControl(SclDocument SclxmlDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            xmlDocument = SclxmlDocument.XmlDocument;

            XmlAttribute nameAttr = xmlNode.Attributes["name"];

            if (nameAttr == null)
                SclxmlDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "GSEControl has no name attribute", this, "MissingName");
        }

        public SclGSEControl(XmlDocument xmlDoc, string name)
        {
            xmlDocument = xmlDoc;
            xmlNode = xmlDoc.CreateElement("GSEControl", SclDocument.SCL_XMLNS);

            XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", name);

            XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "appID", name);
        }

        public SclGSEControl(XmlDocument xmlDocument)
        {
            this.xmlDocument = xmlDocument;
            xmlNode = xmlDocument.CreateElement("scl:GSEControl");
        }

        public string Name
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "name");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "name", value);
            }
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

        public string DatSet
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "datSet");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "datSet", value);
            }
        }

        public string AppID
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "appID");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "appID", value);
            }
        }

        public int ConfRev
        {
            get
            {
                string valStr = XmlHelper.GetAttributeValue(xmlNode, "confRev");

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
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "confRev", value.ToString());
            }
        }

        public bool FixedOffs
        {
            get
            {
                return XmlHelper.ParseBooleanAttribute(xmlNode, "fixedOffs", false);
            }
            set
            {
                XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "fixedOffs", value);
            }
        }

        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "type", value);
            }
        }

        public string SecurityEnabled
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "securityEnabled");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "securityEnabled", value);
            }
        }
    }
}
