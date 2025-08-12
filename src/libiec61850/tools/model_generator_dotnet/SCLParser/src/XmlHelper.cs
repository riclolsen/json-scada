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
    internal class XmlHelper
    {
        public static XmlAttribute GetAttributeCreateIfNotExist(XmlDocument xmlDoc, XmlNode node, string attrName)
        {
            XmlAttribute xmlAttr = node.Attributes[attrName];

            if (xmlAttr == null)
            {
                xmlAttr = xmlDoc.CreateAttribute(attrName);
                node.Attributes.Append(xmlAttr);
            }

            return xmlAttr;
        }

        public static void
        SetAttributeCreateIfNotExists(XmlDocument xmlDoc, XmlNode node, string attrName, string value)
        {
            XmlAttribute xmlAttr = GetAttributeCreateIfNotExist(xmlDoc, node, attrName);

            xmlAttr.Value = value;
        }

        public static void
        SetBooleanAttributeCreateIfNotExists(XmlDocument xmlDoc, XmlNode node, string attrName, bool value)
        {
            string strVal;

            if (value)
                strVal = "true";
            else
                strVal = "false";

            SetAttributeCreateIfNotExists(xmlDoc, node, attrName, strVal);
        }


        public static string GetAttributeValue(XmlNode node, string attrName)
        {
            string value = null;

            XmlAttribute xmlAttr = node.Attributes[attrName];

            if (xmlAttr != null)
                value = xmlAttr.Value;

            return value;
        }


        public static bool
        ParseBooleanAttribute(XmlNode xmlNode, string attributeName, bool defaultValue)
        {
            XmlAttribute attr = xmlNode.Attributes[attributeName];

            bool attrVal = defaultValue;

            if (attr != null)
            {
                if (Boolean.TryParse(attr.Value, out attrVal) == false)
                    throw new SclParserException(xmlNode, attributeName + ": failed to parse boolean attribute");
            }

            return attrVal;
        }
    }

}

