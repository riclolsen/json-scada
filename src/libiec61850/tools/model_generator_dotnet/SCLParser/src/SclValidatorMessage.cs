/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

using System.Xml.Schema;

namespace IEC61850.SCL
{
    public class SclValidatorMessage
    {
        private XmlSeverityType level;
        private string message;
        private int lineNo = -1;
        private int linePos = -1;

        public XmlSeverityType Level
        {
            get
            {
                return level;
            }
        }

        public string Message
        {
            get
            {
                return message;
            }
        }

        public int LineNo
        {
            get
            {
                return lineNo;
            }
        }

        public int LinePos
        {
            get
            {
                return linePos;
            }
        }

        public SclValidatorMessage(XmlSeverityType level, string message)
        {
            this.level = level;
            this.message = message;
        }

        public SclValidatorMessage(XmlSeverityType level, string message, int lineNo, int linePos)
            : this(level, message)
        {
            this.lineNo = lineNo;
            this.linePos = linePos;
        }
    }

}
