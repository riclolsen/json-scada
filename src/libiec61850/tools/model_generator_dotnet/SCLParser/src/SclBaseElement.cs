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
    public class SclBaseElement
    {
        protected SclDocument sclDocument;

        public XmlNode xmlNode;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public SclBaseElement(XmlDocument xmlDocument, SclDocument sclDocument, XmlNode xmlNode)
        {
            this.xmlNode = xmlNode;
            this.sclDocument = sclDocument;
        }

        /// <summary>
        /// Gets the attribute value.
        /// </summary>
        /// <returns>The attribute value.</returns>
        /// <param name="attributeName">Attribute name.</param>
        public string GetAttributeValue(string attributeName)
        {
            return XmlHelper.GetAttributeValue(xmlNode, attributeName);
        }

        /// <summary>
        /// Sets the attribute value.
        /// </summary>
        /// <param name="attributeName">Attribute name.</param>
        /// <param name="value">Value.</param>
        public void SetAttributeValue(string attributeName, string value)
        {
            XmlHelper.SetAttributeCreateIfNotExists(sclDocument.XmlDocument, xmlNode, attributeName, value);
        }

        /// <summary>
        /// Gets or sets the text content of the element
        /// </summary>
        /// <value>the text content</value>
        public string Text
        {
            get
            {
                return xmlNode.InnerText;
            }

            set
            {
                xmlNode.InnerText = value;
            }
        }
    }
}
