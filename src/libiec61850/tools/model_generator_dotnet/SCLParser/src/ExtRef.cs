/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */
using System.Xml;

namespace IEC61850.SCL
{
    public class SclExtRef
    {
        public XmlNode xmlNode;
        private XmlDocument xmlDocument;
        public Inputs Parent;

        //public XmlNode XmlNode
        //{
        //    get { return xmlNode; }
        //}

        public SclExtRef(SclDocument sclDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            xmlDocument = sclDocument.XmlDocument;
        }

        public string IedName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "iedName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "iedName", value);
            }
        }

        public string LdInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "ldInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ldInst", value);
            }
        }

        public string Prefix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "prefix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "prefix", value);
            }
        }

        public string LnClass
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnClass");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnClass", value);
            }
        }

        public string LnInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "lnInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "lnInst", value);
            }
        }

        public string DoName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "doName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "doName", value);
            }
        }

        public string DaName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "daName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "daName", value);
            }
        }

        public string IntAddr
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "intAddr");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "intAddr", value);
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

        public string ServiceType
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "serviceType");

            }
            set
            {
                if (value == null)
                {
                    XmlAttribute xmlAttribute = xmlNode.Attributes["serviceType"];
                    if (xmlAttribute != null)
                        xmlNode.Attributes.Remove(xmlAttribute);
                }
                else
                    XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "serviceType", value);
                PServT = value;
            }
        }

        public string SrcLDInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "srcLDInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "srcLDInst", value);
            }
        }

        public string SrcPrefix
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "srcPrefix");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "srcPrefix", value);
            }
        }

        public string SrcLNClass
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "srcLNClass");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "srcLNClass", value);
            }
        }

        public string SrcLNInst
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "srcLNInst");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "srcLNInst", value);
            }
        }

        public string SrcCBName
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "srcCBName");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "srcCBName", value);
            }
        }

        public string PDO
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "pDO");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pDO", value);
            }
        }

        public string PLN
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "pLN");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pLN", value);
            }
        }

        public string PDA
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "pDA");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pDA", value);
            }
        }

        public string PServT
        {

            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "pServT");
            }
            set
            {

                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pServT", value);

            }
        }

    }
}
