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
    public class SclPrivate : SclBaseElement
    {
        internal SclPrivate(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode)
             : base(xmlDocument, sclDocument, xmlNode)
        {
        }

        public string Type
        {
            get
            {
                return XmlHelper.GetAttributeValue(xmlNode, "type");
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(sclDocument.XmlDocument, xmlNode, "type", value);
            }
        }
    }
}
