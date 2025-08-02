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
    public class SclServices
    {
        public enum BufMode
        {
            UNBUFFERED,
            BUFFERED,
            BOTH
        }

        public XmlNode XmlNode = null;

        private XmlDocument xmlDocument = null;

        private SclDocument sclDocument = null;

        private bool dynAssociationExists = false;
        private int dynAssociationMax = -1; /* -1 => not available */

        private bool settingGroupsExists = false;
        private bool sgEditExists = false;
        private bool confSgExists = false;
        private bool sgEditResvTms = false;
        private bool confSgResvTms = false;

        private bool getDirectoryExists = false;
        private bool getDataObjectDefinitionExists = false;
        private bool getDataSetValueExists = false;
        private bool setDataSetValueExists = false;
        private bool dataSetDirectoryExists = false;

        private bool confDataSetExists = false;
        private int confDataSetMax = 0; /* maximum number of data sets including the preconfigured ones */
        private int confDataSetMaxAttributes = -1; /* maximum number of data attributes in a data set */
        private bool confDataSetModify = true; /* preconfigured data sets may be modified */

        private bool dynDataSetExists = false;
        private int dynDataSetMax = -1; /* maximum number of dynamic data sets including the preconfigured ones */
        private int dynDataSetMaxAttributes = -1;  /* maximum number of data attributes in a data set */

        private bool readWriteExists = false; /* GetData, SetData, Operate */
        private bool timerActivatedControlExists;

        private bool confReportControlExists = false; /* Capability of static (by configuration via SCL) creation of report control blocks */
        private int confReportControlMax = 0; /* maximum number of RCBs including preconfigured ones */
        private int confReportControlMaxBuf = 0; /* he maximum number of instantiable buffered control blocks. If it is missing, its value is equal to the max value. If supplied, its value shall be smaller than the max value */
        private bool confReportControlBufConf = false; /* if the buffered attribute of a preconfigured report control block can be changed via SCL */
        private BufMode confReportControlBufMode = BufMode.BOTH;
        private bool getCBValuesExists = false;

        private bool confLogControlExists = false; /* Capability of static creation of log control blocks */
        private int confLogControlMax = 0;

        private bool reportSettingsExists = false; /* Capability of online setting of RCB attributes */
        private string reportSettingsCbName = null;
        private string reportSettingsDatSet = null;
        private string reportSettingsRptID = null;
        private string reportSettingsOptFields = null;
        private string reportSettingsBufTime = null;
        private string reportSettingsTrgOps = null;
        private string reportSettingsIntgPd = null;
        private bool reportSettingsResvTms = false;
        private bool reportSettingsOwner = false;

        private bool logSettingsExists = false; /* Capability of online setting of LCB attributes */
        private string logSettingsCbName = null;
        private string logSettingsDatSet = null;
        private string logSettingsLogEna = null;
        private string logSettingsTrgOps = null;
        private string logSettingsIntgPd = null;

        private bool gseSettingsExists = false; /* Capability of online setting of GSE-CB attributes */
        private bool gseSettingsCbName = false;
        private string gseCbName = null;
        private string gseDataSet = null;
        private string gseAppID = null;
        private string gseDataLabel = null;
        private bool gseSettingsDatSet = false;
        private bool gseSettingsKdaParticipant = false;
        private bool gseSettingsMcSecurity = false;
        private bool gseSettingsAppId = false;
        private bool gseSettingsDataLabel = false; /* only for GSSE control blocks */

        private bool smvSettingsExists = false; /* Capability of online setting of SMV-CB attributes */
        private string smvSettingsCbName = null;
        private string smvSettingsDatSet = null;
        private string smvSettingsSvId = null;
        private string smvSettingsOptFields = null;
        private string smvSettingsSmpRate = null;
        private string smvSettingsNofASDU = null;
        private bool smvSettingsSamplesPerSec = false;
        private bool smvSettingsSynchrSrcId = false;
        private bool smvSettingsPdcTimeStamp = false;
        private bool smvSettingsKdaParticipant = false;
        private bool smvSettingsSmpRateElement = false;
        private int smvSettingsSmpRateElementVal = 0;
        private bool smvSettingsSamplesPerSecElement = false;
        private int smvSettingsSamplesPerSecElementVal = 0;
        private bool smvSettingsSecPerSamplesElement = false;
        private int smvSettingsSecPerSamplesElementVal = 0;

        private bool confLNsExists = false; /* Defines what can be configured for LNs defined in an ICD file */
        private bool confLNsFixPrefix = true;
        private bool confLNsFixLnInst = true;

        private bool confLdNameExists = false;

        private bool valueHandlingExists = false;

        private bool gseDirExists = false;

        private bool gooseExists = false;
        private bool gooseFixedOffs = false;
        private bool gooseGoose = false;
        private bool gooseRGOOSE = false;
        private int gooseMax = 0; /* max number of GOOSE CBs that are configurable for publishing; 0 = only client */

        private bool gsseExists = false;
        private int gsseMax = 0; /* max number of GSSE CBs that are configurable for publishing; 0 = only client */

        private bool smvScExists = false; /* IED can be a SMV server or client */
        private bool smvScDeliveryConf = false;
        private bool smvScSv = false;
        private bool smvScRSV = false;
        private int smvScMax = 0; /*  max number of SMV CBs that are configurable for publishing; 0 = only client */

        private bool fileHandlingExists = false;
        private bool fileHandlingFtps = false;
        private bool fileHandlingMms = false;
        private bool fileHandlingFtp = false;

        private bool commProtExists = false;
        private bool commProtIpv6 = false;

        private bool timeSyncProtExists = false;
        private bool timeSyncProtOther = false;
        private bool timeSyncProtIc61850_9_3 = false;
        private bool timeSyncProtC37_238 = false;
        private bool timeSyncProtSntp = false;

        private bool redProtExists = false;
        private bool redProtRstp = false;
        private bool redProtPrp = false;
        private bool redProtHsr = false;

        private bool mcSecurityExists = false;
        private bool mcSecurityEncryption = false;
        private bool mcSecuritySignature = false;

        private bool dataObjectDirectoryExists = false;

        private bool supSubscriptionExists = false; /* Capability to supervise GOOSE or SMV subscriptions */
        //private int supSubscriptionMaxGO = 0; /* maximum number of subscription supervision LNs to be instantiated in the IED */
        private int supSubscriptionMaxGo = 0;
        private int supSubscriptionMaxSv = 0;

        private bool confSigRefExists = false; /* Capability to include input references into logical nodes */
        private int confSigRefMax = 0;

        private int nameLength = 32;


        public bool DynDataSetExists
        {
            get
            {
                return dynDataSetExists;
            }
            set
            {
                dynDataSetExists = value;
            }
        }
        public DynDataSet DynDataSet
        {
            get { return dynDataSet; }
            set { dynDataSet = value; }
        }
        public bool DeleteDynDataSetServices()
        {
            if (dynDataSet != null)
            {
                XmlNode.RemoveChild(dynDataSet.XmlNode);
                dynDataSet = null;
                DynDataSetExists = false;

                return true;

            }

            return false;
        }

        public int DynDataSetMax
        {
            get
            {
                return dynDataSetMax;
            }
            set
            {
                dynDataSetMax = value;
            }
        }
        public int DynDataSetMaxAttributes
        {
            get
            {
                return dynDataSetMaxAttributes;
            }
            set
            {
                dynDataSetMaxAttributes = value;
            }
        }


        public bool ReadWriteExists
        {
            get
            {
                return readWriteExists;
            }
            set
            {
                readWriteExists = value;
            }
        }
        public ReadWrite ReadWrite
        {
            get { return readWrite; }
            set { readWrite = value; }
        }
        public bool DeleteReadWriteServices()
        {
            if (readWrite != null)
            {
                XmlNode.RemoveChild(readWrite.XmlNode);
                readWrite = null;
                ReadWriteExists = false;

                return true;

            }

            return false;
        }

        public bool TimerActivatedControlExists
        {
            get
            {
                return timerActivatedControlExists;
            }
            set
            {
                timerActivatedControlExists = value;
            }
        }
        public TimerActivatedControl TimerActivatedControl
        {
            get { return timerActivatedControl; }
            set { timerActivatedControl = value; }
        }
        public bool DeleteTimerActivatedControlServices()
        {
            if (timerActivatedControl != null)
            {
                XmlNode.RemoveChild(timerActivatedControl.XmlNode);
                timerActivatedControl = null;
                TimerActivatedControlExists = false;

                return true;

            }

            return false;
        }

        public bool ConfReportControlExists
        {
            get
            {
                return confReportControlExists;
            }
            set
            {
                confReportControlExists = value;
            }
        }
        public ConfReportControl ConfReportControl
        {
            get { return confReportControl; }
            set { confReportControl = value; }
        }
        public bool DeleteConfReportControlServices()
        {
            if (confReportControl != null)
            {
                XmlNode.RemoveChild(confReportControl.XmlNode);
                confReportControl = null;
                ConfReportControlExists = false;

                return true;

            }

            return false;
        }

        public int ConfReportControlMax
        {
            get
            {
                return confReportControlMax;
            }
            set
            {
                confReportControlMax = value;
            }
        }
        public int ConfReportControlMaxBuf
        {
            get
            {
                return confReportControlMaxBuf
;
            }
            set
            {
                confReportControlMaxBuf = value;
            }
        }
        public BufMode ConfReportControlBufMode
        {
            get
            {
                return confReportControlBufMode;
            }
            set
            {
                confReportControlBufMode = value;
            }
        }
        public bool ConfReportControlBufConf
        {
            get
            {
                return confReportControlBufConf;
            }
            set
            {
                confReportControlBufConf = value;
            }
        }



        public bool GetCBValuesExists
        {
            get
            {
                return getCBValuesExists;
            }
            set
            {
                getCBValuesExists = value;
            }
        }
        public GetCBValues GetCBValues
        {
            get { return getCBValues; }
            set { getCBValues = value; }
        }
        public bool DeleteGetCBValuesServices()
        {
            if (getCBValues != null)
            {
                XmlNode.RemoveChild(getCBValues.XmlNode);
                getCBValues = null;
                GetCBValuesExists = false;

                return true;

            }

            return false;
        }

        public bool ConfLogControlExists
        {
            get
            {
                return confLogControlExists;
            }
            set
            {
                confLogControlExists = value;
            }
        }
        public ConfLogControl ConfLogControl
        {
            get { return confLogControl; }
            set { confLogControl = value; }
        }
        public bool DeleteConfLogControlServices()
        {
            if (confLogControl != null)
            {
                XmlNode.RemoveChild(confLogControl.XmlNode);
                confLogControl = null;
                ConfLogControlExists = false;

                return true;

            }

            return false;
        }

        public int ConfLogControlMax
        {
            get
            {
                return confLogControlMax;
            }
            set
            {
                confLogControlMax = value;
            }
        }



        public bool ReportSettingsExists
        {
            get
            {
                return reportSettingsExists;
            }
            set
            {
                reportSettingsExists = value;
            }
        }
        public ReportSettings ReportSettings
        {
            get { return reportSettings; }
            set { reportSettings = value; }
        }
        public bool DeleteReportSettingsServices()
        {
            if (reportSettings != null)
            {
                XmlNode.RemoveChild(reportSettings.XmlNode);
                reportSettings = null;
                ReportSettingsExists = false;

                return true;

            }

            return false;
        }

        public string ReportSettingsCbName
        {
            get
            {
                return reportSettingsCbName;
            }
            set
            {
                reportSettingsCbName = value;
            }
        }
        public string ReportSettingsDatSet
        {
            get
            {
                return reportSettingsDatSet;
            }
            set
            {
                reportSettingsDatSet = value;
            }
        }
        public string ReportSettingsRptID
        {
            get
            {
                return reportSettingsRptID;
            }
            set
            {
                reportSettingsRptID = value;
            }
        }
        public string ReportSettingsOptFields
        {
            get
            {
                return reportSettingsOptFields;
            }
            set
            {
                reportSettingsOptFields = value;
            }
        }
        public string ReportSettingsBufTime
        {
            get
            {
                return reportSettingsBufTime;
            }
            set
            {
                reportSettingsBufTime = value;
            }
        }
        public string ReportSettingsTrgOps
        {
            get
            {
                return reportSettingsTrgOps;
            }
            set
            {
                reportSettingsTrgOps = value;
            }
        }
        public string ReportSettingsIntgPd
        {
            get
            {
                return reportSettingsIntgPd;
            }
            set
            {
                reportSettingsIntgPd = value;
            }
        }
        public bool ReportSettingsResvTms
        {
            get
            {
                return reportSettingsResvTms;
            }
            set
            {
                reportSettingsResvTms = value;
            }
        }
        public bool ReportSettingsOwner
        {
            get
            {
                return reportSettingsOwner;
            }
            set
            {
                reportSettingsOwner = value;
            }
        }



        public bool LogSettingsExists
        {
            get
            {
                return logSettingsExists;
            }
            set
            {
                logSettingsExists = value;
            }
        }
        public LogSettings LogSettings
        {
            get { return logSettings; }
            set { logSettings = value; }
        }
        public bool DeleteLogSettingsServices()
        {
            if (logSettings != null)
            {
                XmlNode.RemoveChild(logSettings.XmlNode);
                logSettings = null;
                LogSettingsExists = false;

                return true;

            }

            return false;
        }

        public string LogSettingsCbName
        {
            get
            {
                return logSettingsCbName;
            }
            set
            {
                logSettingsCbName = value;
            }
        }
        public string LogSettingsDatSet
        {
            get
            {
                return logSettingsDatSet;
            }
            set
            {
                logSettingsDatSet = value;
            }
        }
        public string LogSettingsLogEna
        {
            get
            {
                return logSettingsLogEna;
            }
            set
            {
                logSettingsLogEna = value;
            }
        }
        public string LogSettingsTrgOps
        {
            get
            {
                return logSettingsTrgOps;
            }
            set
            {
                logSettingsTrgOps = value;
            }
        }
        public string LogSettingsIntgPd
        {
            get
            {
                return logSettingsIntgPd;
            }
            set
            {
                logSettingsIntgPd = value;
            }
        }



        public bool GseSettingsExists
        {
            get
            {
                return gseSettingsExists;
            }
            set
            {
                gseSettingsExists = value;
            }
        }
        public GSESettings GSESettings
        {
            get { return gseSettings; }
            set { gseSettings = value; }

        }
        public bool DeleteGSESettingsServices()
        {
            if (gseSettings != null)
            {
                XmlNode.RemoveChild(gseSettings.XmlNode);
                gseSettings = null;
                GseSettingsExists = false;

                return true;

            }

            return false;
        }

        public string GseCbName
        {
            get
            {
                return gseCbName;
            }
            set
            {
                gseCbName = value;
            }
        }
        public string GseDatSet
        {
            get
            {
                return gseDataSet;
            }
            set
            {
                gseDataSet = value;
            }
        }
        public string GseAppID
        {
            get
            {
                return gseAppID;
            }
            set
            {
                gseAppID = value;
            }
        }
        public string GseDataLabel
        {
            get
            {
                return gseDataLabel;
            }
            set
            {
                gseDataLabel = value;
            }
        }
        public bool GseSettingsKdaParticipant
        {
            get
            {
                return gseSettingsKdaParticipant;
            }
            set
            {
                gseSettingsKdaParticipant = value;
            }
        }
        public bool GseSettingsMcSecurity
        {
            get
            {
                return gseSettingsMcSecurity;
            }
            set
            {
                gseSettingsMcSecurity = value;
            }
        }
        public bool GseSettingsCbName
        {
            get
            {
                return gseSettingsCbName;
            }
            set
            {
                gseSettingsCbName = value;
            }
        }
        public bool GseSettingsDatSet
        {
            get
            {
                return gseSettingsDatSet;
            }
            set
            {
                gseSettingsDatSet = value;
            }
        }
        public bool GseSettingsAppId
        {
            get
            {
                return gseSettingsAppId;
            }
            set
            {
                gseSettingsAppId = value;
            }
        }
        public bool GseSettingsDataLabel
        {
            get
            {
                return gseSettingsDataLabel;
            }
            set
            {
                gseSettingsDataLabel = value;
            }
        }



        public bool SmvSettingsExists
        {
            get
            {
                return smvSettingsExists;
            }
            set
            {
                smvSettingsExists = value;
            }
        }
        public SMVSettings SMVSettings
        {
            get { return smvSettings; }
            set { smvSettings = value; }
        }
        public bool DeleteSMVSettingsServices()
        {
            if (smvSettings != null)
            {
                XmlNode.RemoveChild(smvSettings.XmlNode);
                smvSettings = null;
                SmvSettingsExists = false;

                return true;

            }

            return false;
        }

        public string SmvSettingsCbName
        {
            get
            {
                return smvSettingsCbName;
            }
            set
            {
                smvSettingsCbName = value;
            }
        }
        public string SmvSettingsDatSet
        {
            get
            {
                return smvSettingsDatSet;
            }
            set
            {
                smvSettingsDatSet = value;
            }
        }
        public string SmvSettingsSvId
        {
            get
            {
                return smvSettingsSvId;
            }
            set
            {
                smvSettingsSvId = value;
            }
        }
        public string SmvSettingsOptFields
        {
            get
            {
                return smvSettingsOptFields;
            }
            set
            {
                smvSettingsOptFields = value;
            }
        }
        public string SmvSettingsSmpRate
        {
            get
            {
                return smvSettingsSmpRate;
            }
            set
            {
                smvSettingsSmpRate = value;
            }
        }
        public string SmvSettingsNofASDU
        {
            get
            {
                return smvSettingsNofASDU;
            }
            set
            {
                smvSettingsNofASDU = value;
            }
        }
        public bool SmvSettingsSamplesPerSec
        {
            get
            {
                return smvSettingsSamplesPerSec;
            }
            set
            {
                smvSettingsSamplesPerSec = value;
            }
        }
        public bool SmvSettingsSynchrSrcId
        {
            get
            {
                return smvSettingsSynchrSrcId;
            }
            set
            {
                smvSettingsSynchrSrcId = value;
            }
        }
        public bool SmvSettingsPdcTimeStamp
        {
            get
            {
                return smvSettingsPdcTimeStamp;
            }
            set
            {
                smvSettingsPdcTimeStamp = value;
            }
        }
        public bool SmvSettingsKdaParticipant
        {
            get
            {
                return smvSettingsKdaParticipant;
            }
            set
            {
                smvSettingsKdaParticipant = value;
            }
        }
        public bool SmvSettingsSamplesPerSecElement
        {
            get
            {
                return smvSettingsSamplesPerSecElement;
            }
            set
            {
                smvSettingsSamplesPerSecElement = value;
            }
        }
        public int SmvSettingsSamplesPerSecElementVal
        {
            get
            {
                return smvSettingsSamplesPerSecElementVal;
            }
            set
            {
                smvSettingsSamplesPerSecElementVal = value;
            }
        }
        public bool SmvSettingsSmpRateElement
        {
            get
            {
                return smvSettingsSmpRateElement;
            }
            set
            {
                smvSettingsSmpRateElement = value;
            }
        }
        public int SmvSettingsSmpRateElementVal
        {
            get
            {
                return smvSettingsSmpRateElementVal;
            }
            set
            {
                smvSettingsSmpRateElementVal = value;
            }
        }
        public bool SmvSettingsSecPerSamplesElement
        {
            get
            {
                return smvSettingsSecPerSamplesElement;
            }
            set
            {
                smvSettingsSecPerSamplesElement = value;
            }
        }
        public int SmvSettingsSecPerSamplesElementVal
        {
            get
            {
                return smvSettingsSecPerSamplesElementVal;
            }
            set
            {
                smvSettingsSecPerSamplesElementVal = value;
            }
        }


        public bool ConfLNsExists
        {
            get
            {
                return confLNsExists;
            }
            set
            {
                confLNsExists = value;
            }
        }
        public ConfLNs ConfLNs
        {
            get { return confLNs; }
            set { confLNs = value; }
        }
        public bool DeleteConfLNsServices()
        {
            if (confLNs != null)
            {
                XmlNode.RemoveChild(confLNs.XmlNode);
                confLNs = null;
                ConfLNsExists = false;

                return true;

            }

            return false;
        }

        public bool ConfLNsFixPrefix
        {
            get
            {
                return confLNsFixPrefix;
            }
            set
            {
                confLNsFixPrefix = value;
            }
        }
        public bool ConfLNsFixLnInst
        {
            get
            {
                return confLNsFixLnInst;
            }
            set
            {
                confLNsFixLnInst = value;
            }
        }


        public bool ConfLdNameExists
        {
            get
            {
                return confLdNameExists;
            }
            set
            {
                confLdNameExists = value;
            }
        }
        public ConfLdName ConfLdName
        {
            get { return confLdName; }
            set { confLdName = value; }
        }
        public bool DeleteConfLdNameServices()
        {
            if (confLdName != null)
            {
                XmlNode.RemoveChild(confLdName.XmlNode);
                confLdName = null;
                ConfLdNameExists = false;

                return true;

            }

            return false;
        }

        public bool GseDirExists
        {
            get
            {
                return gseDirExists;
            }
            set
            {
                gseDirExists = value;
            }


        }
        public GSEDir GSEDir
        {
            get { return gseDir; }
            set { gseDir = value; }
        }
        public bool DeleteGSEDirServices()
        {
            if (gseDir != null)
            {
                XmlNode.RemoveChild(gseDir.XmlNode);
                gseDir = null;
                GseDirExists = false;

                return true;

            }

            return false;
        }

        public bool GooseExists
        {
            get
            {
                return gooseExists;
            }
            set
            {
                gooseExists = value;
            }
        }
        public GOOSE GOOSE
        {
            get { return goose; }
            set { goose = value; }
        }
        public bool DeleteGOOSEServices()
        {
            if (goose != null)
            {
                XmlNode.RemoveChild(goose.XmlNode);
                goose = null;
                GooseExists = false;

                return true;

            }

            return false;
        }

        public bool GooseFixedOffs
        {
            get
            {
                return gooseFixedOffs;
            }
            set
            {
                gooseFixedOffs = value;
            }
        }
        public bool GooseGoose
        {
            get
            {
                return gooseGoose;
            }
            set
            {
                gooseGoose = value;
            }
        }
        public bool GooseRGOOSE
        {
            get
            {
                return gooseRGOOSE;
            }
            set
            {
                gooseRGOOSE = value;
            }
        }
        public int GooseMax
        {
            get
            {
                return gooseMax;
            }
            set
            {
                gooseMax = value;
            }
        }



        public bool GsseExists
        {
            get
            {
                return gsseExists;
            }
            set
            {
                gsseExists = value;
            }
        }
        public GSSE GSSE
        {
            get { return gsse; }
            set { gsse = value; }
        }
        public bool DeleteGSSEServices()
        {
            if (gsse != null)
            {
                XmlNode.RemoveChild(gsse.XmlNode);
                gsse = null;
                GsseExists = false;

                return true;

            }

            return false;
        }

        public int GsseMax
        {
            get
            {
                return gsseMax;
            }
            set
            {
                gsseMax = value;
            }
        }



        public bool SmvScExists
        {
            get
            {
                return smvScExists;
            }
            set
            {
                smvScExists = value;
            }
        }
        public SMVsc SMVsc
        {
            get { return sMVsc; }
            set { sMVsc = value; }
        }
        public bool DeleteSMVscServices()
        {
            if (sMVsc != null)
            {
                XmlNode.RemoveChild(sMVsc.XmlNode);
                sMVsc = null;
                SmvScExists = false;

                return true;

            }

            return false;
        }

        public bool SmvScDeliveryConf
        {
            get
            {
                return smvScDeliveryConf;
            }
            set
            {
                smvScDeliveryConf = value;
            }
        }
        public bool SmvScSv
        {
            get
            {
                return smvScSv;
            }
            set
            {
                smvScSv = value;
            }
        }
        public bool SmvScRSV
        {
            get
            {
                return smvScRSV;
            }
            set
            {
                smvScRSV = value;
            }
        }
        public int SmvScMax
        {
            get
            {
                return smvScMax;
            }
            set
            {
                smvScMax = value;
            }
        }



        public bool FileHandlingExists
        {
            get
            {
                return fileHandlingExists;
            }
            set
            {
                fileHandlingExists = value;
            }
        }
        public FileHandling FileHandling
        {
            get { return fileHandling; }
            set { fileHandling = value; }
        }
        public bool DeleteFileHandlingServices()
        {
            if (fileHandling != null)
            {
                XmlNode.RemoveChild(fileHandling.XmlNode);
                fileHandling = null;
                FileHandlingExists = false;

                return true;

            }

            return false;
        }

        public bool FileHandlingFtps
        {
            get
            {
                return fileHandlingFtps;
            }
            set
            {
                fileHandlingFtps = value;
            }
        }
        public bool FileHandlingMms
        {
            get
            {
                return fileHandlingMms;
            }
            set
            {
                fileHandlingMms = value;
            }
        }
        public bool FileHandlingFtp
        {
            get
            {
                return fileHandlingFtp;
            }
            set
            {
                fileHandlingFtp = value;
            }
        }



        public bool SupSubscriptionExists
        {
            get
            {
                return supSubscriptionExists;
            }
            set
            {
                supSubscriptionExists = value;
            }
        }
        public SupSubscription SupSubscription
        {
            get { return supSubscription; }
            set { supSubscription = value; }
        }
        public bool DeleteSupSubscriptionServices()
        {
            if (supSubscription != null)
            {
                XmlNode.RemoveChild(supSubscription.XmlNode);
                supSubscription = null;
                SupSubscriptionExists = false;

                return true;

            }

            return false;
        }

        public int SupSubscriptionMaxGo
        {
            get
            {
                return supSubscriptionMaxGo;
            }
            set
            {
                supSubscriptionMaxGo = value;
            }
        }
        public int SupSubscriptionMaxSv
        {
            get
            {
                return supSubscriptionMaxSv;
            }
            set
            {
                supSubscriptionMaxSv = value;
            }
        }



        public bool ConfSigRefExists
        {
            get
            {
                return confSigRefExists;
            }
            set
            {
                confSigRefExists = value;
            }
        }
        public ConfSigRef ConfSigRef
        {
            get { return confSigRef; }
            set { confSigRef = value; }
        }
        public bool DeleteConfSigRefServices()
        {
            if (confSigRef != null)
            {
                XmlNode.RemoveChild(confSigRef.XmlNode);
                confSigRef = null;
                ConfSigRefExists = false;

                return true;

            }

            return false;
        }

        public int ConfSigRefMax
        {
            get
            {
                return confSigRefMax;
            }
            set
            {
                confSigRefMax = value;
            }
        }



        public bool CommProtExists
        {
            get
            {
                return commProtExists;
            }
            set
            {
                commProtExists = value;
            }
        }
        public CommProt CommProt
        {
            get { return commProt; }
            set { commProt = value; }
        }
        public bool DeleteCommProtServices()
        {
            if (commProt != null)
            {
                XmlNode.RemoveChild(commProt.XmlNode);
                commProt = null;
                CommProtExists = false;

                return true;

            }

            return false;
        }
        public bool CommProtIpv6
        {
            get
            {
                return commProtIpv6;
            }
            set
            {
                commProtIpv6 = value;
            }
        }



        public bool TimeSyncProtExists
        {
            get
            {
                return timeSyncProtExists;
            }
            set
            {
                timeSyncProtExists = value;
            }
        }
        public TimeSyncProt TimeSyncProt
        {
            get { return timeSyncProt; }
            set { timeSyncProt = value; }
        }
        public bool DeleteTimeSyncProtServices()
        {
            if (timeSyncProt != null)
            {
                XmlNode.RemoveChild(timeSyncProt.XmlNode);
                timeSyncProt = null;
                TimeSyncProtExists = false;

                return true;

            }

            return false;
        }

        public bool TimeSyncProtC37_238
        {
            get
            {
                return timeSyncProtC37_238;
            }
            set
            {
                timeSyncProtC37_238 = value;
            }
        }
        public bool TimeSyncProtIc61850_9_3
        {
            get
            {
                return timeSyncProtIc61850_9_3;
            }
            set
            {
                timeSyncProtIc61850_9_3 = value;
            }
        }
        public bool TimeSyncProtOther
        {
            get
            {
                return timeSyncProtOther;
            }
            set
            {
                timeSyncProtOther = value;
            }
        }
        public bool TimeSyncProtSntp
        {
            get
            {
                return timeSyncProtSntp;
            }
            set
            {
                timeSyncProtSntp = value;
            }
        }



        public bool RedProtExists
        {
            get
            {
                return redProtExists;
            }
            set
            {
                redProtExists = value;
            }
        }
        public RedProt RedProt
        {
            get { return redProt; }
            set { redProt = value; }
        }
        public bool DeleteRedProtServices()
        {
            if (redProt != null)
            {
                XmlNode.RemoveChild(redProt.XmlNode);
                redProt = null;
                RedProtExists = false;

                return true;

            }

            return false;
        }

        public bool RedProtHsr
        {
            get
            {
                return redProtHsr;
            }
            set
            {
                redProtHsr = value;
            }
        }
        public bool RedProtPrp
        {
            get
            {
                return redProtPrp;
            }
            set
            {
                redProtPrp = value;
            }
        }
        public bool RedProtRstp
        {
            get
            {
                return redProtRstp;
            }
            set
            {
                redProtRstp = value;
            }
        }


        public bool ValueHandlingExists
        {
            get
            {
                return valueHandlingExists;
            }
            set
            {
                valueHandlingExists = value;
            }
        }
        public ValueHandling ValueHandling
        {
            get { return valueHandling; }
            set { valueHandling = value; }
        }
        public bool DeleteValueHandlingServices()
        {
            if (valueHandling != null)
            {
                XmlNode.RemoveChild(valueHandling.XmlNode);
                valueHandling = null;
                ValueHandlingExists = false;

                return true;

            }

            return false;
        }

        public bool McSecurityExists
        {
            get
            {
                return mcSecurityExists;
            }
            set
            {
                mcSecurityExists = value;
            }
        }
        public McSecurity McSecurity
        {
            get { return mcSecurity; }
            set { mcSecurity = value; }
        }
        public bool DeleteMcSecurityServices()
        {
            if (mcSecurity != null)
            {
                XmlNode.RemoveChild(mcSecurity.XmlNode);
                mcSecurity = null;
                McSecurityExists = false;
                return true;

            }

            return false;
        }

        public bool McSecurityEncryption
        {
            get
            {
                return mcSecurityEncryption;
            }
            set
            {
                mcSecurityEncryption = value;
            }
        }
        public bool McSecuritySignature
        {
            get
            {
                return mcSecuritySignature;
            }
            set
            {
                mcSecuritySignature = value;
            }
        }




        public bool DataObjectDirectoryExists
        {
            get
            {
                return dataObjectDirectoryExists;
            }
            set
            {
                dataObjectDirectoryExists = value;
            }
        }
        public DataObjectDirectory DataObjectDirectory
        {
            get { return dataObjectDirectory; }
            set { dataObjectDirectory = value; }
        }
        public bool DeleteDataObjectDirectoryServices()
        {
            if (dataObjectDirectory != null)
            {
                XmlNode.RemoveChild(dataObjectDirectory.XmlNode);
                dataObjectDirectory = null;
                DataObjectDirectoryExists = false;
                return true;

            }

            return false;
        }

        public int NameLength
        {
            get
            {
                return nameLength;
            }
            set
            {
                nameLength = value;
            }
        }


        public ClientServices ClientServices
        {
            get
            {
                return clientServices;
            }
            set
            {
                clientServices = value;
            }


        }
        public bool DeleteClientServices()
        {
            if (clientServices != null)
            {
                XmlNode.RemoveChild(clientServices.XmlNode);
                clientServices = null;


                return true;

            }

            return false;
        }

        public bool DynAssociationExists
        {
            get
            {
                return dynAssociationExists;
            }
            set
            {
                dynAssociationExists = value;
            }
        }
        public DynAssociation DynAssociation
        {
            get
            {
                return dynAssociation;
            }
            set
            {
                dynAssociation = value;
            }
        }
        public bool DeleteDynAssociation()
        {
            if (dynAssociation != null)
            {
                XmlNode.RemoveChild(dynAssociation.XmlNode);
                dynAssociation = null;
                dynAssociationExists = false;

                return true;

            }

            return false;
        }

        public int DynAssociationMax
        {
            get
            {
                return dynAssociationMax;
            }
            set
            {
                dynAssociationMax = value;
            }
        }



        public bool SettingGroupsExists
        {
            get
            {
                return settingGroupsExists;
            }
            set
            {
                settingGroupsExists = value;
            }
        }
        public SettingGroups SettingGroups
        {
            get
            {
                return settingGroups;
            }
            set
            {
                settingGroups = value;
            }
        }
        public bool SgEditExists
        {
            get
            {
                return sgEditExists;
            }
            set
            {
                sgEditExists = value;
            }
        }
        public bool SgEditResvTms
        {
            get
            {
                return sgEditResvTms;
            }
            set
            {
                sgEditResvTms = value;
            }
        }
        public bool ConfSgExists
        {
            get
            {
                return confSgExists;
            }
            set
            {
                confSgExists = value;
            }
        }
        public bool ConfSgResvTms
        {
            get
            {
                return confSgResvTms;
            }
            set
            {
                confSgResvTms = value;
            }
        }
        public bool DeleteSettingGroups()
        {
            if (settingGroups != null)
            {
                XmlNode.RemoveChild(settingGroups.XmlNode);

                settingGroups = null;
                settingGroupsExists = false;

                return true;

            }

            return false;
        }

        public bool GetDirectoryExists
        {
            get
            {
                return getDirectoryExists;
            }
            set
            {
                getDirectoryExists = value;
            }


        }
        public GetDirectory GetDirectory
        {
            get { return getDirectory; }
            set { getDirectory = value; }
        }
        public bool DeleteGetDirectoryServices()
        {
            if (getDirectory != null)
            {
                XmlNode.RemoveChild(getDirectory.XmlNode);
                getDirectory = null;
                getDirectoryExists = false;
                return true;

            }

            return false;
        }

        public bool GetDataObjectDefinitionExists
        {
            get
            {
                return getDataObjectDefinitionExists;
            }
            set
            {
                getDataObjectDefinitionExists = value;
            }


        }
        public GetDataObjectDefinition GetDataObjectDefinition
        {
            get { return getDataObjectDefinition; }
            set { getDataObjectDefinition = value; }
        }
        public bool DeleteGetDataObjectDefinitionServices()
        {
            if (getDataObjectDefinition != null)
            {
                XmlNode.RemoveChild(getDataObjectDefinition.XmlNode);
                getDataObjectDefinition = null;
                GetDataObjectDefinitionExists = false;
                return true;
            }
            return false;
        }

        public bool GetDataSetValueExists
        {
            get
            {
                return getDataSetValueExists;
            }
            set
            {
                getDataSetValueExists = value;
            }


        }
        public GetDataSetValue GetDataSetValue
        {
            get { return getDataSetValue; }
            set { getDataSetValue = value; }
        }
        public bool DeleteGetDataSetValueServices()
        {
            if (getDataSetValue != null)
            {
                XmlNode.RemoveChild(getDataSetValue.XmlNode);
                getDataSetValue = null;
                GetDataSetValueExists = false;
                return true;
            }
            return false;
        }

        public bool SetDataSetValueExists
        {
            get
            {
                return setDataSetValueExists;
            }
            set
            {
                setDataSetValueExists = value;
            }

        }
        public SetDataSetValue SetDataSetValue
        {
            get { return setDataSetValue; }
            set { setDataSetValue = value; }
        }
        public bool DeleteSetDataSetValueServices()
        {
            if (setDataSetValue != null)
            {
                XmlNode.RemoveChild(setDataSetValue.XmlNode);
                setDataSetValue = null;
                SetDataSetValueExists = false;
                return true;
            }
            return false;
        }


        public bool DataSetDirectoryExists
        {
            get
            {
                return dataSetDirectoryExists;
            }
            set
            {
                dataSetDirectoryExists = value;
            }


        }
        public DataSetDirectory DataSetDirectory
        {
            get { return dataSetDirectory; }
            set { dataSetDirectory = value; }
        }
        public bool DeleteDataSetDirectoryServices()
        {
            if (dataSetDirectory != null)
            {
                XmlNode.RemoveChild(dataSetDirectory.XmlNode);
                dataSetDirectory = null;
                DataSetDirectoryExists = false;
                return true;
            }
            return false;
        }

        public bool ConfDataSetExists
        {
            get
            {
                return confDataSetExists;
            }
            set
            {
                confDataSetExists = value;
            }
        }
        public ConfDataSet ConfDataSet
        {
            get { return confDataSet; }
            set { confDataSet = value; }
        }
        public bool DeleteConfDataSetServices()
        {
            if (confDataSet != null)
            {
                XmlNode.RemoveChild(confDataSet.XmlNode);
                confDataSet = null;
                ConfDataSetExists = false;
                return true;
            }
            return false;
        }

        public int ConfDataSetMax
        {
            get
            {
                return confDataSetMax;
            }
            set
            {
                confDataSetMax = value;
            }
        }
        public int ConfDataSetMaxAttributes
        {
            get
            {
                return confDataSetMaxAttributes;
            }
            set
            {
                confDataSetMaxAttributes = value;
            }
        }
        public bool ConfDataSetModify
        {
            get
            {
                return confDataSetModify;
            }
            set
            {
                confDataSetModify = value;
            }
        }


        private ClientServices clientServices = null;
        private TimeSyncProt timeSyncProt = null;
        private McSecurity mcSecurity = null;
        private DynAssociation dynAssociation = null;
        private SettingGroups settingGroups = null;
        private GetDirectory getDirectory = null;
        private GetDataObjectDefinition getDataObjectDefinition = null;
        private DataObjectDirectory dataObjectDirectory = null;
        private GetDataSetValue getDataSetValue = null;
        private SetDataSetValue setDataSetValue = null;
        private DataSetDirectory dataSetDirectory = null;
        private ConfDataSet confDataSet = null;
        private DynDataSet dynDataSet = null;
        private ReadWrite readWrite = null;
        private TimerActivatedControl timerActivatedControl = null;
        private ConfReportControl confReportControl = null;
        private GetCBValues getCBValues = null;
        private ConfLogControl confLogControl = null;
        private ReportSettings reportSettings = null;
        private LogSettings logSettings = null;
        private GSESettings gseSettings = null;
        private SMVSettings smvSettings = null;
        private ConfLNs confLNs = null;
        private ConfLdName confLdName = null;
        private GSEDir gseDir = null;
        private GOOSE goose = null;
        private GSSE gsse = null;
        private SMVsc sMVsc = null;
        private FileHandling fileHandling = null;
        private SupSubscription supSubscription = null;
        private ConfSigRef confSigRef = null;
        private CommProt commProt = null;
        private RedProt redProt = null;
        private ValueHandling valueHandling = null;


        public SclServices(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            XmlNode = xmlNode;
            xmlDocument = xmlDocument;
            this.sclDocument = sclDocument;

            string nameLengthAttribute = XmlHelper.GetAttributeValue(xmlNode, "nameLength");
            if (nameLengthAttribute != null)
            {
                if (int.TryParse(nameLengthAttribute, out int intNameLength))
                {
                    if (intNameLength != 64 && intNameLength != 32)
                    {
                        sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "NameLength attribute on IED services should be 32 default or 64. Value is " + nameLengthAttribute, this, "nameLength");
                        nameLength = intNameLength;
                    }
                }
                else
                {
                    sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "NameLength attribute on IED services is not a integer. Value is " + nameLengthAttribute, this, "nameLength");
                }

            }
            else
            {
                sclDocument.AddIssue(xmlNode, "ERROR", "Model integrity", "No nameLength attribute on IED services", this, "nameLength");
            }

            XmlNode clientServices = xmlNode.SelectSingleNode("scl:ClientServices", nsManager);
            if (clientServices != null)
            {
                ClientServices = new ClientServices(sclDocument, clientServices, nsManager);
            }


            XmlNode DynAssociationNode = xmlNode.SelectSingleNode("scl:DynAssociation", nsManager);
            if (DynAssociationNode != null)
            {
                DynAssociationExists = true;
                DynAssociation = new DynAssociation(sclDocument, DynAssociationNode, nsManager);

            }


            XmlNode SettingGroupsNode = xmlNode.SelectSingleNode("scl:SettingGroups", nsManager);
            if (SettingGroupsNode != null)
            {
                SettingGroupsExists = true;
                SettingGroups = new SettingGroups(sclDocument, SettingGroupsNode, nsManager);

                //XmlNode SGEditNode = SettingGroupsNode.SelectSingleNode("scl:SGEdit", nsManager);
                //if (SGEditNode != null)
                //{
                //    SgEditExists = true;

                //    string resvTms = XmlHelper.GetAttributeValue(SGEditNode, "resvTms");
                //    if (resvTms != null)
                //    {
                //        if (resvTms == "true")
                //            SgEditResvTms = true;
                //        else
                //            SgEditResvTms = false;


                //    }


                //}
                //else
                //    SgEditExists = false;

                //XmlNode confSGNode = SettingGroupsNode.SelectSingleNode("scl:ConfSG", nsManager);
                //if (confSGNode != null)
                //{
                //    ConfSgExists = true;

                //    string resvTms = XmlHelper.GetAttributeValue(confSGNode, "resvTms");
                //    if (resvTms != null)
                //    {
                //        if (resvTms == "true")
                //            ConfSgResvTms = true;
                //        else
                //            ConfSgResvTms = false;


                //    }


                //}
                //else
                //    ConfSgExists = false;

            }


            XmlNode GetDirectoryNode = xmlNode.SelectSingleNode("scl:GetDirectory", nsManager);
            if (GetDirectoryNode != null)
            {
                GetDirectoryExists = true;
                GetDirectory = new GetDirectory(sclDocument, GetDirectoryNode, nsManager);
            }


            XmlNode GetDataObjectDefinitionNode = xmlNode.SelectSingleNode("scl:GetDataObjectDefinition", nsManager);
            if (GetDataObjectDefinitionNode != null)
            {
                GetDataObjectDefinitionExists = true;
                GetDataObjectDefinition = new GetDataObjectDefinition(sclDocument, GetDataObjectDefinitionNode, nsManager);
            }


            XmlNode DataObjectDirectoryNode = xmlNode.SelectSingleNode("scl:DataObjectDirectory", nsManager);
            if (DataObjectDirectoryNode != null)
            {
                DataObjectDirectoryExists = true;
                DataObjectDirectory = new DataObjectDirectory(sclDocument, DataObjectDirectoryNode, nsManager);
            }


            XmlNode GetDataSetValueNode = xmlNode.SelectSingleNode("scl:GetDataSetValue", nsManager);
            if (GetDataSetValueNode != null)
            {
                GetDataSetValueExists = true;
                GetDataSetValue = new GetDataSetValue(sclDocument, GetDataSetValueNode, nsManager);
            }


            XmlNode SetDataSetValueNode = xmlNode.SelectSingleNode("scl:SetDataSetValue", nsManager);
            if (SetDataSetValueNode != null)
            {
                SetDataSetValueExists = true;
                SetDataSetValue = new SetDataSetValue(sclDocument, SetDataSetValueNode, nsManager);
            }


            XmlNode DataSetDirectoryNode = xmlNode.SelectSingleNode("scl:DataSetDirectory", nsManager);
            if (DataSetDirectoryNode != null)
            {
                DataSetDirectoryExists = true;
                DataSetDirectory = new DataSetDirectory(sclDocument, DataSetDirectoryNode, nsManager);
            }


            XmlNode ConfDataSetNode = xmlNode.SelectSingleNode("scl:ConfDataSet", nsManager);
            if (ConfDataSetNode != null)
            {
                ConfDataSetExists = true;
                ConfDataSet = new ConfDataSet(sclDocument, ConfDataSetNode, nsManager);

                //string attribute = XmlHelper.GetAttributeValue(ConfDataSetNode, "max");
                //if (attribute != null)
                //    ConfDataSetMax = int.Parse(attribute);

                //string maxAttributes = XmlHelper.GetAttributeValue(ConfDataSetNode, "maxAttributes");
                //if (maxAttributes != null)
                //    ConfDataSetMaxAttributes = int.Parse(maxAttributes);

                //string modify = XmlHelper.GetAttributeValue(ConfDataSetNode, "modify");
                //if (modify != null)
                //{
                //    if (modify == "true")
                //        ConfDataSetModify = true;

                //}

            }


            XmlNode DynDataSetNode = xmlNode.SelectSingleNode("scl:DynDataSet", nsManager);
            if (DynDataSetNode != null)
            {
                DynDataSetExists = true;
                DynDataSet = new DynDataSet(sclDocument, DynDataSetNode, nsManager);

                //string max = XmlHelper.GetAttributeValue(DynDataSetNode, "max");
                //if (max != null)
                //    DynDataSetMax = int.Parse(max);

                //string maxAttributes = XmlHelper.GetAttributeValue(DynDataSetNode, "maxAttributes");
                //if (maxAttributes != null)
                //    DynDataSetMaxAttributes = int.Parse(maxAttributes);

            }


            XmlNode ReadWriteNode = xmlNode.SelectSingleNode("scl:ReadWrite", nsManager);
            if (ReadWriteNode != null)
            {
                ReadWriteExists = true;
                ReadWrite = new ReadWrite(sclDocument, ReadWriteNode, nsManager);
            }


            XmlNode TimerActivatedControlNode = xmlNode.SelectSingleNode("scl:TimerActivatedControl", nsManager);
            if (TimerActivatedControlNode != null)
            {
                TimerActivatedControlExists = true;
                TimerActivatedControl = new TimerActivatedControl(sclDocument, TimerActivatedControlNode, nsManager);
            }


            XmlNode ConfReportControlNode = xmlNode.SelectSingleNode("scl:ConfReportControl", nsManager);
            if (ConfReportControlNode != null)
            {
                ConfReportControlExists = true;
                ConfReportControl = new ConfReportControl(sclDocument, ConfReportControlNode, nsManager);
                //string max = XmlHelper.GetAttributeValue(ConfReportControlNode, "max");
                //if (max != null)
                //    ConfReportControlMax = int.Parse(max);

                //string bufMode = XmlHelper.GetAttributeValue(ConfReportControlNode, "bufMode");
                //if (bufMode != null)
                //    if (bufMode == "UNBUFFERED")
                //        ConfReportControlBufMode = BufMode.UNBUFFERED;
                //    else if (bufMode == "BUFFERED")
                //        ConfReportControlBufMode = BufMode.BUFFERED;
                //    else
                //        ConfReportControlBufMode = BufMode.BOTH;

                //string bufConf = XmlHelper.GetAttributeValue(ConfReportControlNode, "bufConf");
                //if (bufConf != null)
                //    if (bufConf == "true")
                //        ConfReportControlBufConf = true;


                //string maxBuf = XmlHelper.GetAttributeValue(ConfReportControlNode, "maxBuf");
                //if (maxBuf != null)
                //    ConfReportControlMaxBuf = int.Parse(maxBuf);


            }


            XmlNode GetCBValuesNode = xmlNode.SelectSingleNode("scl:GetCBValues", nsManager);
            if (GetCBValuesNode != null)
            {
                GetCBValuesExists = true;
                GetCBValues = new GetCBValues(sclDocument, GetCBValuesNode, nsManager);
            }


            XmlNode ConfLogControlNode = xmlNode.SelectSingleNode("scl:ConfLogControl", nsManager);
            if (ConfLogControlNode != null)
            {
                ConfLogControlExists = true;
                ConfLogControl = new ConfLogControl(sclDocument, ConfLogControlNode, nsManager);

                //string max = XmlHelper.GetAttributeValue(ConfLogControlNode, "max");

                //if (max != null)
                //    ConfLogControlMax = int.Parse(max);
            }


            XmlNode ReportSettingsNode = xmlNode.SelectSingleNode("scl:ReportSettings", nsManager);
            if (ReportSettingsNode != null)
            {
                ReportSettingsExists = true;
                ReportSettings = new ReportSettings(sclDocument, ReportSettingsNode, nsManager);

                //string cbName = XmlHelper.GetAttributeValue(ReportSettingsNode, "cbName");
                //if (cbName != null)
                //    ReportSettingsCbName = cbName;

                //string datSet = XmlHelper.GetAttributeValue(ReportSettingsNode, "datSet");
                //if (datSet != null)
                //    ReportSettingsDatSet = datSet;

                //string rptID = XmlHelper.GetAttributeValue(ReportSettingsNode, "rptID");
                //if (rptID != null)
                //    ReportSettingsRptID = rptID;

                //string optFields = XmlHelper.GetAttributeValue(ReportSettingsNode, "optFields");
                //if (optFields != null)
                //    ReportSettingsOptFields = optFields;

                //string bufTime = XmlHelper.GetAttributeValue(ReportSettingsNode, "bufTime");
                //if (bufTime != null)
                //    ReportSettingsBufTime = bufTime;

                //string trgOps = XmlHelper.GetAttributeValue(ReportSettingsNode, "trgOps");
                //if (trgOps != null)
                //    ReportSettingsTrgOps = trgOps;

                //string intgPd = XmlHelper.GetAttributeValue(ReportSettingsNode, "intgPd");
                //if (intgPd != null)
                //    ReportSettingsIntgPd = intgPd;

                //string resvTms = XmlHelper.GetAttributeValue(ReportSettingsNode, "resvTms");
                //if (resvTms != null)
                //{
                //    if (resvTms == "true")
                //        ReportSettingsResvTms = true;
                //}

                //string owner = XmlHelper.GetAttributeValue(ReportSettingsNode, "owner");
                //if (owner != null)
                //{
                //    if (owner == "true")
                //        ReportSettingsOwner = true;
                //}
            }


            XmlNode LogSettingsNode = xmlNode.SelectSingleNode("scl:LogSettings", nsManager);
            if (LogSettingsNode != null)
            {
                LogSettingsExists = true;
                LogSettings = new LogSettings(sclDocument, LogSettingsNode, nsManager);
                //string cbName = XmlHelper.GetAttributeValue(LogSettingsNode, "cbName");
                //if (cbName != null)
                //    LogSettingsCbName = cbName;

                //string datSet = XmlHelper.GetAttributeValue(LogSettingsNode, "datSet");
                //if (datSet != null)
                //    LogSettingsDatSet = datSet;

                //string logEna = XmlHelper.GetAttributeValue(LogSettingsNode, "logEna");
                //if (logEna != null)
                //    LogSettingsLogEna = logEna;

                //string trgOps = XmlHelper.GetAttributeValue(LogSettingsNode, "trgOps");
                //if (trgOps != null)
                //    LogSettingsTrgOps = trgOps;

                //string intgPd = XmlHelper.GetAttributeValue(LogSettingsNode, "intgPd");
                //if (intgPd != null)
                //    LogSettingsIntgPd = intgPd;

            }


            XmlNode GSEControlNode = xmlNode.SelectSingleNode("scl:GSESettings", nsManager);
            if (GSEControlNode != null)
            {
                GseSettingsExists = true;
                GSESettings = new GSESettings(sclDocument, GSEControlNode, nsManager);

                //string cbName = XmlHelper.GetAttributeValue(GSEControlNode, "cbName");
                //if (cbName != null)
                //{
                //    GseCbName = cbName;
                //    GseSettingsCbName = true;
                //}

                //string datSet = XmlHelper.GetAttributeValue(GSEControlNode, "datSet");
                //if (datSet != null)
                //{
                //    GseDatSet = datSet;
                //    GseSettingsDatSet = true;
                //}

                //string appID = XmlHelper.GetAttributeValue(GSEControlNode, "appID");
                //if (datSet != null)
                //{
                //    GseAppID = appID;
                //    GseSettingsAppId = true;
                //}

                //string dataLabel = XmlHelper.GetAttributeValue(GSEControlNode, "dataLabel");
                //if (dataLabel != null)
                //{
                //    GseDataLabel = dataLabel;
                //    GseSettingsDataLabel = true;
                //}

                //string kdaParticipant = XmlHelper.GetAttributeValue(GSEControlNode, "kdaParticipant");
                //if (kdaParticipant != null)
                //{
                //    if (kdaParticipant == "true")
                //        GseSettingsKdaParticipant = true;
                //}

                //XmlNode McSecurity = GSEControlNode.SelectSingleNode("scl:McSecurity", nsManager);

                //if(McSecurity != null)
                //{
                //    GseSettingsMcSecurity = true;
                //}
            }


            XmlNode SMVSettingsNode = xmlNode.SelectSingleNode("scl:SMVSettings", nsManager);
            if (SMVSettingsNode != null)
            {
                SmvSettingsExists = true;
                SMVSettings = new SMVSettings(sclDocument, SMVSettingsNode, nsManager);

                //string cbName = XmlHelper.GetAttributeValue(SMVSettingsNode, "cbName");
                //if (cbName != null)
                //    SmvSettingsCbName = cbName;

                //string datSet = XmlHelper.GetAttributeValue(SMVSettingsNode, "datSet");
                //if (datSet != null)
                //    SmvSettingsDatSet = datSet;

                //string svID = XmlHelper.GetAttributeValue(SMVSettingsNode, "svID");
                //if (svID != null)
                //    SmvSettingsSvId = svID;

                //string optFields = XmlHelper.GetAttributeValue(SMVSettingsNode, "optFields");
                //if (optFields != null)
                //    SmvSettingsOptFields = optFields;

                //string smpRate = XmlHelper.GetAttributeValue(SMVSettingsNode, "smpRate");
                //if (smpRate != null)
                //    SmvSettingsSmpRate = smpRate;

                //string samplesPerSec = XmlHelper.GetAttributeValue(SMVSettingsNode, "samplesPerSec");
                //if (samplesPerSec != null)
                //{
                //    if (samplesPerSec == "true")
                //        SmvSettingsSamplesPerSec = true;

                //}

                //string synchrSrcId = XmlHelper.GetAttributeValue(SMVSettingsNode, "synchrSrcId");
                //if (synchrSrcId != null)
                //{
                //    if (synchrSrcId == "true")
                //        SmvSettingsSynchrSrcId = true;

                //}

                //string nofASDU = XmlHelper.GetAttributeValue(SMVSettingsNode, "nofASDU");
                //if (nofASDU != null)
                //    SmvSettingsNofASDU = nofASDU;

                //string pdcTimeStamp = XmlHelper.GetAttributeValue(SMVSettingsNode, "pdcTimeStamp");
                //if (pdcTimeStamp != null)
                //{
                //    if (pdcTimeStamp == "true")
                //        SmvSettingsPdcTimeStamp = true;

                //}

                //string kdaParticipant = XmlHelper.GetAttributeValue(SMVSettingsNode, "kdaParticipant");
                //if (kdaParticipant != null)
                //{
                //    if (kdaParticipant == "true")
                //        SmvSettingsKdaParticipant = true;

                //}

                //XmlNode SmpRate = SMVSettingsNode.SelectSingleNode("scl:SmpRate", nsManager);
                //if (SmpRate != null)
                //{
                //    SmvSettingsSmpRateElement = true;

                //    string val = SmpRate.InnerText;
                //    if (val != null)
                //        SmvSettingsSmpRateElementVal = int.Parse(val);
                //}

                //XmlNode SamplesPerSec = SMVSettingsNode.SelectSingleNode("scl:SamplesPerSec", nsManager);
                //if (SamplesPerSec != null)
                //{
                //    SmvSettingsSamplesPerSecElement = true;

                //    string val = SamplesPerSec.InnerText;
                //    if (val != null)
                //        SmvSettingsSamplesPerSecElementVal = int.Parse(val);
                //}

                //XmlNode SecPerSamples = SMVSettingsNode.SelectSingleNode("scl:SecPerSamples", nsManager);
                //if (SecPerSamples != null)
                //{
                //    SmvSettingsSecPerSamplesElement = true;

                //    string val = SecPerSamples.InnerText;
                //    if (val != null)
                //        SmvSettingsSecPerSamplesElementVal = int.Parse(val);
                //}
            }


            XmlNode ConfLNsNode = xmlNode.SelectSingleNode("scl:ConfLNs", nsManager);
            if (ConfLNsNode != null)
            {
                ConfLNsExists = true;
                ConfLNs = new ConfLNs(sclDocument, ConfLNsNode, nsManager);

                //string fixPrefix = XmlHelper.GetAttributeValue(ConfLNsNode, "fixPrefix");
                //if (fixPrefix != null)
                //{
                //    if (fixPrefix == "true")
                //        ConfLNsFixPrefix = true;

                //}

                //string fixLnInst = XmlHelper.GetAttributeValue(ConfLNsNode, "fixLnInst");
                //if (fixLnInst != null)
                //{
                //    if (fixLnInst == "true")
                //        ConfLNsFixLnInst = true;

                //}

            }


            XmlNode ConfLdNameNode = xmlNode.SelectSingleNode("scl:ConfLdName", nsManager);
            if (ConfLdNameNode != null)
            {
                ConfLdNameExists = true;
                ConfLdName = new ConfLdName(sclDocument, ConfLdNameNode, nsManager);

            }


            XmlNode GSEDirNode = xmlNode.SelectSingleNode("scl:GSEDir", nsManager);
            if (GSEDirNode != null)
            {
                GseDirExists = true;
                GSEDir = new GSEDir(sclDocument, GSEDirNode, nsManager);

            }


            XmlNode GOOSENode = xmlNode.SelectSingleNode("scl:GOOSE", nsManager);
            if (GOOSENode != null)
            {
                GooseExists = true;
                GOOSE = new GOOSE(sclDocument, GOOSENode, nsManager);

                //string max = XmlHelper.GetAttributeValue(GOOSENode, "max");
                //if (max != null)
                //{
                //    GooseMax = int.Parse(max);
                //}

                //string fixedOffs = XmlHelper.GetAttributeValue(GOOSENode, "fixedOffs");
                //if (fixedOffs != null)
                //{
                //    if (fixedOffs == "true")
                //        GooseFixedOffs = true;

                //}

                //string goose = XmlHelper.GetAttributeValue(GOOSENode, "goose");
                //if (goose != null)
                //{
                //    if (goose == "true")
                //        GooseGoose = true;

                //}

                //string rGOOSE = XmlHelper.GetAttributeValue(GOOSENode, "rGOOSE");
                //if (rGOOSE != null)
                //{
                //    if (rGOOSE == "true")
                //        GooseRGOOSE = true;

                //}


            }


            XmlNode GSSENode = xmlNode.SelectSingleNode("scl:GSSE", nsManager);
            if (GSSENode != null)
            {
                GsseExists = true;
                GSSE = new GSSE(sclDocument, GSSENode, nsManager);

                //string max = XmlHelper.GetAttributeValue(GSSENode, "max");
                //if (max != null)
                //{
                //    GsseMax = int.Parse(max);
                //}


            }


            XmlNode SMVscNode = xmlNode.SelectSingleNode("scl:SMVsc", nsManager);
            if (SMVscNode != null)
            {
                SmvScExists = true;
                SMVsc = new SMVsc(sclDocument, SMVscNode, nsManager);

                //string max = XmlHelper.GetAttributeValue(SMVscNode, "max");
                //if (max != null)
                //{
                //    SmvScMax = int.Parse(max);
                //}

                //string deliveryConf = XmlHelper.GetAttributeValue(ConfLNsNode, "deliveryConf");
                //if (deliveryConf != null)
                //{
                //    if (deliveryConf == "true")
                //        SmvScDeliveryConf = true;
                //    else
                //        SmvScDeliveryConf = false;
                //}

                //string sv = XmlHelper.GetAttributeValue(ConfLNsNode, "sv");
                //if (sv != null)
                //{
                //    if (sv == "true")
                //        SmvScSv = true;
                //    else
                //        SmvScSv = false;
                //}

                //string rSV = XmlHelper.GetAttributeValue(ConfLNsNode, "rSV");
                //if (rSV != null)
                //{
                //    if (rSV == "true")
                //        SmvScRSV = true;
                //    else
                //        SmvScRSV = false;
                //}


            }


            XmlNode FileHandlingNode = xmlNode.SelectSingleNode("scl:FileHandling", nsManager);
            if (FileHandlingNode != null)
            {
                FileHandlingExists = true;
                FileHandling = new FileHandling(sclDocument, FileHandlingNode, nsManager);

                //string mms = XmlHelper.GetAttributeValue(FileHandlingNode, "mms");
                //if (mms != null)
                //{
                //    if (mms == "true")
                //        FileHandlingMms = true;
                //    else
                //        FileHandlingMms = false;
                //}

                //string ftp = XmlHelper.GetAttributeValue(FileHandlingNode, "ftp");
                //if (ftp != null)
                //{
                //    if (ftp == "true")
                //        FileHandlingFtp = true;
                //    else
                //        FileHandlingFtp = false;
                //}

                //string ftps = XmlHelper.GetAttributeValue(FileHandlingNode, "ftps");
                //if (ftps != null)
                //{
                //    if (ftps == "true")
                //        FileHandlingFtps = true;
                //    else
                //        FileHandlingFtps = false;
                //}


            }


            XmlNode SupSubscriptionNode = xmlNode.SelectSingleNode("scl:SupSubscription", nsManager);
            if (SupSubscriptionNode != null)
            {
                SupSubscriptionExists = true;
                SupSubscription = new SupSubscription(sclDocument, SupSubscriptionNode, nsManager);

                //string maxGo = XmlHelper.GetAttributeValue(SupSubscriptionNode, "maxGo");
                //if (maxGo != null)
                //{
                //    SupSubscriptionMaxGo = int.Parse(maxGo);
                //}

                //string maxSv = XmlHelper.GetAttributeValue(SupSubscriptionNode, "maxSv");
                //if (maxSv != null)
                //{
                //    SupSubscriptionMaxSv = int.Parse(maxSv);
                //}


            }


            XmlNode ConfSigRefNode = xmlNode.SelectSingleNode("scl:ConfSigRef", nsManager);
            if (ConfSigRefNode != null)
            {
                ConfSigRefExists = true;
                ConfSigRef = new ConfSigRef(sclDocument, ConfSigRefNode, nsManager);

                //string max = XmlHelper.GetAttributeValue(ConfSigRefNode, "max");
                //if (max != null)
                //{
                //    ConfSigRefMax = int.Parse(max);
                //}

            }


            XmlNode CommProtNode = xmlNode.SelectSingleNode("scl:CommProt", nsManager);
            if (CommProtNode != null)
            {
                CommProtExists = true;
                CommProt = new CommProt(sclDocument, CommProtNode, nsManager);

                //string ipv6 = XmlHelper.GetAttributeValue(FileHandlingNode, "ipv6");
                //if (ipv6 != null)
                //{
                //    if (ipv6 == "true")
                //        CommProtIpv6 = true;
                //    else
                //        CommProtIpv6 = false;
                //}

            }


            XmlNode TimeSyncProtNode = xmlNode.SelectSingleNode("scl:TimeSyncProt", nsManager);
            if (TimeSyncProtNode != null)
            {
                TimeSyncProtExists = true;
                TimeSyncProt = new TimeSyncProt(sclDocument, TimeSyncProtNode, nsManager);
                ////
                //string sntp = XmlHelper.GetAttributeValue(TimeSyncProtNode, "sntp");
                //if (sntp != null)
                //{
                //    if (sntp == "true")
                //        TimeSyncProtSntp = true;
                //    else
                //        TimeSyncProtSntp = false;
                //}

                //string c37_238 = XmlHelper.GetAttributeValue(TimeSyncProtNode, "c37_238");
                //if (c37_238 != null)
                //{
                //    if (c37_238 == "true")
                //        TimeSyncProtC37_238 = true;
                //    else
                //        TimeSyncProtC37_238 = false;
                //}

                //string iec61850_9_3 = XmlHelper.GetAttributeValue(TimeSyncProtNode, "iec61850_9_3");
                //if (iec61850_9_3 != null)
                //{
                //    if (iec61850_9_3 == "true")
                //        TimeSyncProtIc61850_9_3 = true;
                //    else
                //        TimeSyncProtIc61850_9_3 = false;
                //}

                //string other = XmlHelper.GetAttributeValue(TimeSyncProtNode, "other");
                //if (other != null)
                //{
                //    if (other == "true")
                //        TimeSyncProtOther = true;
                //    else
                //        TimeSyncProtOther = false;
                //}

            }


            XmlNode RedProtNode = xmlNode.SelectSingleNode("scl:RedProt", nsManager);
            if (RedProtNode != null)
            {
                RedProtExists = true;
                RedProt = new RedProt(sclDocument, RedProtNode, nsManager);

                //string hsr = XmlHelper.GetAttributeValue(RedProtNode, "hsr");
                //if (hsr != null)
                //{
                //    if (hsr == "true")
                //        RedProtHsr = true;
                //    else
                //        RedProtHsr = false;
                //}

                //string prp = XmlHelper.GetAttributeValue(RedProtNode, "prp");
                //if (prp != null)
                //{
                //    if (prp == "true")
                //        RedProtPrp = true;
                //    else
                //        RedProtPrp = false;
                //}

                //string rstp = XmlHelper.GetAttributeValue(RedProtNode, "rstp");
                //if (rstp != null)
                //{
                //    if (rstp == "true")
                //        RedProtRstp = true;
                //    else
                //        RedProtRstp = false;
                //}

            }


            XmlNode ValueHandlingNode = xmlNode.SelectSingleNode("scl:ValueHandling", nsManager);
            if (ValueHandlingNode != null)
            {
                ValueHandlingExists = true;
                ValueHandling = new ValueHandling(sclDocument, ValueHandlingNode, nsManager);
            }


            XmlNode McSecurityNode = xmlNode.SelectSingleNode("scl:McSecurity", nsManager);
            if (McSecurityNode != null)
            {
                McSecurityExists = true;
                McSecurity = new McSecurity(sclDocument, McSecurityNode, nsManager);
                ////
                //string signature = XmlHelper.GetAttributeValue(McSecurityNode, "signature");
                //if (signature != null)
                //{
                //    if (signature == "true")
                //        McSecuritySignature = true;
                //    else
                //        McSecuritySignature = false;
                //}

                //string encryption = XmlHelper.GetAttributeValue(McSecurityNode, "encryption");
                //if (encryption != null)
                //{
                //    if (encryption == "true")
                //        McSecurityEncryption = true;
                //    else
                //        McSecurityEncryption = false;
                //}
            }

        }

    }



    public class ClientServices
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public int MaxAttributes
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxAttributes");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxAttributes", value.ToString());

            }
        }

        public int MaxReports
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxReports");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxReports", value.ToString());

            }
        }

        public int MaxGOOSE
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxGOOSE");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxGOOSE", value.ToString());

            }
        }

        public int MaxSMV
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxSMV");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxSMV", value.ToString());

            }
        }

        public bool RGOOSE
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "rGOOSE", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rGOOSE", value.ToString()); }
        }

        public bool RSV
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "rSV", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rSV", value.ToString()); }
        }

        public bool NoIctBinding
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "noIctBinding", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "noIctBinding", value.ToString()); }
        }

        public TimeSyncProt TimeSyncProt = null;
        public McSecurity McSecurity = null;


        public void DeleteMcSecurity()
        {
            if (McSecurity != null)
            {
                XmlNode.RemoveChild(McSecurity.XmlNode);
                McSecurity = null;
            }

        }
        public void DeleteTimeSyncProt()
        {
            if (TimeSyncProt != null)
            {
                XmlNode.RemoveChild(TimeSyncProt.XmlNode);
                TimeSyncProt = null;
            }

        }

        public ClientServices(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlNode timeSync = xmlNode.SelectSingleNode("scl:TimeSyncProt", nsManager);

            if (timeSync != null)
            {
                TimeSyncProt = new TimeSyncProt(sclDocument, timeSync, nsManager);
            }

            XmlNode mcSecurity = xmlNode.SelectSingleNode("scl:McSecurity", nsManager);

            if (mcSecurity != null)
            {
                McSecurity = new McSecurity(sclDocument, mcSecurity, nsManager);
            }
        }

    }
    public class TimeSyncProt
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public bool Sntp
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "sntp", true); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "sntp", value.ToString()); }
        }

        public bool C37_238
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "c37_238", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "c37_238", value.ToString()); }
        }

        public bool Iec61850_9_3
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "iec61850_9_3", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "iec61850_9_3", value.ToString()); }
        }

        public bool Other
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "other", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "other", value.ToString()); }
        }

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public TimeSyncProt(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class McSecurity
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public bool Signature
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "signature", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "signature", value.ToString()); }
        }

        public bool Encryption
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "encryption", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "encryption", value.ToString()); }
        }


        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public McSecurity(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class DynAssociation
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }

        public DynAssociation(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


        }

    }
    public class SettingGroups
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        bool sgEditExists = false;
        bool confSgExists = false;
        bool confSgResvTms = false;
        bool sgEditResvTms = false;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public bool SgEditExists
        {
            get
            {
                return sgEditExists;
            }
            set
            {
                sgEditExists = value;
            }
        }
        public bool ConfSgExists
        {
            get
            {
                return confSgExists;
            }
            set
            {
                confSgExists = value;
            }
        }
        public bool SgEditResvTms
        {
            get
            {
                return sgEditResvTms;
            }
            set
            {
                sgEditResvTms = value;
            }
        }
        public bool ConfSgResvTms
        {
            get
            {
                return confSgResvTms;
            }
            set
            {
                confSgResvTms = value;
            }
        }

        public SGEdit SGEdit = null;
        public ConfSG ConfSG = null;
        public SettingGroups(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlNode sGEdit = xmlNode.SelectSingleNode("scl:SGEdit", nsManager);

            if (sGEdit != null)
            {
                SGEdit = new SGEdit(sclDocument, sGEdit, nsManager);
                sgEditExists = true;
            }

            XmlNode confSG = xmlNode.SelectSingleNode("scl:ConfSG", nsManager);

            if (confSG != null)
            {
                ConfSG = new ConfSG(sclDocument, confSG, nsManager);
                confSgExists = true;
            }

        }
    }

    public class SGEdit
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public bool resvTms
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "resvTms", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "resvTms", value.ToString()); }
        }

        public bool SGCB
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "SGCB", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "SGCB", value.ToString()); }
        }

        public bool file
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "file", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "file", value.ToString()); }
        }

        public XmlNode XmlNode { get => xmlNode; set => xmlNode = value; }

        public SGEdit(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class ConfSG
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode { get => xmlNode; set => xmlNode = value; }

        public bool resvTms
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "resvTms", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "resvTms", value.ToString()); }
        }

        public bool SGCB
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "SGCB", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "SGCB", value.ToString()); }
        }

        public bool file
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "file", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "file", value.ToString()); }
        }

        public ConfSG(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }

    public class GetDirectory
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public GetDirectory(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


        }

    }
    public class GetDataObjectDefinition
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }


        public GetDataObjectDefinition(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


        }

    }
    public class DataObjectDirectory
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public DataObjectDirectory(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


        }

    }
    public class GetDataSetValue
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public GetDataSetValue(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class SetDataSetValue
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public SetDataSetValue(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class DataSetDirectory
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public DataSetDirectory(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class ConfDataSet
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }
        public int MaxAttributes
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxAttributes");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxAttributes", value.ToString());

            }
        }
        public bool Modify
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "modify", true); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "modify", value.ToString()); }
        }
        public ConfDataSet(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }


    }
    public class DynDataSet
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }
        public int MaxAttributes
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxAttributes");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxAttributes", value.ToString());

            }
        }

        public DynDataSet(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class ReadWrite
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public ReadWrite(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class TimerActivatedControl
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public TimerActivatedControl(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class ConfReportControl
    {
        public enum BufMode
        {
            UNBUFFERED,
            BUFFERED,
            BOTH
        }
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        private BufMode confReportControlBufMode = BufMode.BOTH;

        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }
        public int MaxBuf
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxBuf");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxBuf", value.ToString());

            }

        }
        public bool bufConf
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "bufConf", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "bufConf", value.ToString()); }
        }
        public BufMode ConfReportControlBufMode
        {
            get
            {
                return confReportControlBufMode;
            }
            set
            {
                confReportControlBufMode = value;
            }
        }

        public ConfReportControl(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class GetCBValues
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public GetCBValues(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class ConfLogControl
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }

        public ConfLogControl(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class ReportSettings
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public enum CBName
        {
            Fix,
            Conf
        }

        public enum DATSet
        {
            Fix,
            Conf,
            Dyn
        }

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        private CBName cbName = CBName.Fix;
        private DATSet datSet = DATSet.Fix;

        public CBName CbName
        {
            get { return cbName; }
            set { cbName = value; }
        }

        public DATSet DatSet
        {
            get { return datSet; }
            set { datSet = value; }
        }


        public string RptID
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "rptID"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rptID", value.ToString()); }
        }
        public string OptFields
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "optFields"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "optFields", value.ToString()); }
        }
        public string BufTime
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "bufTime"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "bufTime", value.ToString()); }
        }
        public string TrgOps
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "trgOps"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "trgOps", value.ToString()); }
        }
        public string IntgPd
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "intgPd"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "intgPd", value.ToString()); }
        }
        public bool ResvTms
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "resvTms", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "resvTms", value.ToString()); }
        }
        public bool Owner
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "owner", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "owner", value.ToString()); }
        }
        public ReportSettings(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class LogSettings
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public enum CBName
        {
            Fix,
            Conf
        }
        public enum DATSet
        {
            Fix,
            Conf,
            Dyn
        }

        private CBName cbName = CBName.Fix;
        private DATSet datSet = DATSet.Fix;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public CBName CbName
        {
            get { return cbName; }
            set { cbName = value; }
        }

        public DATSet DatSet
        {
            get { return datSet; }
            set { datSet = value; }
        }

        public string LogEna
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "logEna"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "logEna", value.ToString()); }
        }
        public string TrgOps
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "trgOps"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "trgOps", value.ToString()); }
        }
        public string IntgPd
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "intgPd"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "intgPd", value.ToString()); }
        }

        public LogSettings(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


        }

    }
    public class GSESettings
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public enum CBName
        {
            Fix,
            Conf
        }
        public enum DATSet
        {
            Fix,
            Conf,
            Dyn
        }

        private CBName cbName = CBName.Fix;
        private DATSet datSet = DATSet.Fix;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public CBName CbName
        {
            get { return cbName; }
            set { cbName = value; }
        }
        public DATSet DatSet
        {
            get { return datSet; }
            set { datSet = value; }
        }
        public string AppID
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "appID"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "appID", value.ToString()); }
        }
        public string DataLabel
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "dataLabel"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "dataLabel", value.ToString()); }
        }
        public bool kdaParticipant
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "kdaParticipant", false); }
            //set { XmlHelper.SetBooleanAttributeCreateIfNotExists(xmlDocument, xmlNode, "kdaParticipant", value); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "kdaParticipant", value.ToString()); }
        }

        public McSecurity McSecurity = null;

        public void DeleteMcSecurity()
        {
            if (McSecurity != null)
            {
                XmlNode.RemoveChild(McSecurity.XmlNode);
                McSecurity = null;
            }

        }
        public GSESettings(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

            XmlNode mcSecurity = xmlNode.SelectSingleNode("scl:McSecurity", nsManager);
            //
            //this.XmlNode.InsertAfter(TimeSyncProt.XmlNode, this.XmlNode.FirstChild);
            //

            if (mcSecurity != null)
            {
                McSecurity = new McSecurity(sclDocument, mcSecurity, nsManager);
            }

        }

    }
    public class SMVSettings
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public enum CBName
        {
            Fix,
            Conf
        }
        public enum DATSet
        {
            Fix,
            Conf,
            Dyn
        }
        public enum SMPRate
        {
            Fix,
            Conf,
            Dyn
        }
        public enum NOFASDU
        {
            Fix,
            Conf
        }

        private CBName cbName = CBName.Fix;
        private DATSet datSet = DATSet.Fix;
        private SMPRate sMPRate = SMPRate.Fix;
        private NOFASDU nOFASDU = NOFASDU.Fix;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public CBName CbName
        {
            get { return cbName; }
            set { cbName = value; }
        }
        public DATSet DatSet
        {
            get { return datSet; }
            set { datSet = value; }
        }
        public SMPRate SmpRate
        {
            get { return sMPRate; }
            set { sMPRate = value; }
        }
        public NOFASDU NofASDU
        {
            get { return nOFASDU; }
            set { nOFASDU = value; }
        }
        public string SvID
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "svID"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "svID", value.ToString()); }
        }
        public string OptFields
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "optFields"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "optFields", value.ToString()); }
        }
        public string smpRate
        {
            get { return XmlHelper.GetAttributeValue(xmlNode, "smpRate"); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "smpRate", value.ToString()); }
        }
        public int SamplesPerSec
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "samplesPerSec");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "samplesPerSec", value.ToString());

            }
        }
        public bool SynchrSrcId
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "synchrSrcId", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "synchrSrcId", value.ToString()); }
        }

        public bool PdcTimeStamp
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "pdcTimeStamp", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "pdcTimeStamp", value.ToString()); }
        }
        public bool KdaParticipant
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "kdaParticipant", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "kdaParticipant", value.ToString()); }
        }
        //SubElements
        public int SmpRateValue
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "SmpRate");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "SmpRate", value.ToString());

            }
        }
        public int SamplesPerSecValue
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "SamplesPerSec");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "SamplesPerSec", value.ToString());

            }
        }
        public int SecPerSamplesValue
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "SecPerSamples");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "SecPerSamples", value.ToString());

            }
        }
        //
        public McSecurity McSecurity = null;


        public void DeleteMcSecurity()
        {
            if (McSecurity != null)
            {
                XmlNode.RemoveChild(McSecurity.XmlNode);
                McSecurity = null;
            }

        }
        public SMVSettings(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;


            XmlNode mcSecurity = xmlNode.SelectSingleNode("scl:McSecurity", nsManager);
            if (mcSecurity != null)
            {
                McSecurity = new McSecurity(sclDocument, mcSecurity, nsManager);
            }
        }

    }
    public class ConfLNs
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public bool FixPrefix
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "fixPrefix", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "fixPrefix", value.ToString()); }
        }
        public bool FixLnInst
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "fixLnInst", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "fixLnInst", value.ToString()); }
        }
        public ConfLNs(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }

    public class ConfLdName
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public ConfLdName(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class GSEDir
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public GSEDir(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class GOOSE
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }
        public bool FixedOffs
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "fixedOffs", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "fixedOffs", value.ToString()); }
        }
        public bool Goose
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "goose", true); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "goose", value.ToString()); }
        }
        public bool RGOOSE
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "rGOOSE", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rGOOSE", value.ToString()); }
        }
        public GOOSE(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class GSSE
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }
        public GSSE(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class SMVsc
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);
                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());
            }
        }
        public bool DeliveryConf
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "deliveryConf", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "deliveryConf", value.ToString()); }
        }
        public bool Sv
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "sv", true); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "sv", value.ToString()); }
        }
        public bool RSV
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "rSV", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rSV", value.ToString()); }
        }

        public SMVsc(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class FileHandling
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public bool Mms
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "mms", true); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "mms", value.ToString()); }
        }
        public bool Ftp
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "ftp", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ftp", value.ToString()); }
        }
        public bool Ftps
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "ftps", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ftps", value.ToString()); }
        }
        public FileHandling(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class SupSubscription
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int MaxGo
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxGo");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);
                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxGo", value.ToString());
            }
        }
        public int MaxSv
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "maxSv");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);
                    return retVal;
                }
                else
                    return -1;
            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "maxSv", value.ToString());
            }
        }
        public SupSubscription(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }


    }
    public class ConfSigRef
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }
        public int Max
        {
            get
            {
                string valueStr = XmlHelper.GetAttributeValue(xmlNode, "max");
                if (valueStr != null)
                {
                    int retVal = -1;
                    int.TryParse(valueStr, out retVal);

                    return retVal;
                }
                else
                    return -1;

            }
            set
            {
                XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "max", value.ToString());

            }
        }


        public ConfSigRef(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class CommProt
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public bool Ipv6
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "ipv6", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "ipv6", value.ToString()); }
        }

        public CommProt(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;
        }

    }
    public class RedProt
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;
        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public bool Hsr
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "hsr", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "hsr", value.ToString()); }
        }
        public bool Prp
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "prp", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "prp", value.ToString()); }
        }
        public bool Rstp
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "rstp", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "rstp", value.ToString()); }
        }

        public RedProt(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }
    public class ValueHandling
    {
        private XmlDocument xmlDocument;
        private XmlNode xmlNode;
        private SclDocument sclDocument;

        public XmlNode XmlNode
        {
            get { return xmlNode; }
        }

        public bool SetToRo
        {
            get { return XmlHelper.ParseBooleanAttribute(xmlNode, "setToRo", false); }
            set { XmlHelper.SetAttributeCreateIfNotExists(xmlDocument, xmlNode, "setToRo", value.ToString()); }
        }

        public ValueHandling(SclDocument sclDocument, XmlNode xmlNode, XmlNamespaceManager nsManager)
        {
            this.sclDocument = sclDocument;
            xmlDocument = sclDocument.XmlDocument;
            this.xmlNode = xmlNode;

        }

    }

}



