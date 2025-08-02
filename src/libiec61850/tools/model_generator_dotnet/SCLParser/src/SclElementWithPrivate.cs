/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Collections.Generic;
using System.Xml;

namespace IEC61850.SCL
{
    public class SclElementWithPrivate : SclBaseElement
    {
        private List<SclPrivate> privateElements = new List<SclPrivate>();

        internal SclElementWithPrivate(SclDocument sclDocument, XmlNode xmlNode)
         : base(sclDocument.XmlDocument, sclDocument, xmlNode)
        {

            XmlNodeList privateNodes = xmlNode.SelectNodes("Private");

            foreach (XmlNode privateNode in privateNodes)
            {
                SclPrivate priv = new SclPrivate(sclDocument.XmlDocument, sclDocument, privateNode);

                if (priv != null)
                {
                    privateElements.Add(priv);
                }
            }
        }

        public List<SclPrivate> PrivateElements
        {
            get
            {
                return new List<SclPrivate>(privateElements);
            }
        }
    }
}
