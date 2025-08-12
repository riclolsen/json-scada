/*
 *  Copyright 2013-2025 Michael Zillgith, MZ Automation GmbH
 *
 *  This file is part of MZ Automation IEC 61850 SDK
 * 
 *  All rights reserved.
 */

namespace modeGenerator_example
{

    /// <summary>
    /// This example shows how to handle a large number of information objects
    /// </summary>
    class MainClass
    {
        public static void Main(string[] args)
        {
            Console.WriteLine("Usage: Static Model (1) \n Dynamic Model (2) \n <generator option> <ICD file>  [-ied  <ied-name>] [-ap <access-point-name>] [-out <output-name>] [-modelprefix <model-prefix>]");

            if (args.Length == 0)
            {
                Console.WriteLine("Parse the arguments \n" +
                    "Usage: Static Model (1) \n Dynamic Model (2) \n <generator option> <ICD file>  [-ied  <ied-name>] [-ap <access-point-name>] [-out <output-name>] [-modelprefix <model-prefix>]");
            }
            else
            {
                string accessPointName = null;
                string iedName = null;
                string icdFile = "ICDFiles//simpleIO_ltrk_tests.icd";
                string outputFileName = "static_model";
                string modelPrefix = "iedModel";
                bool initializeOnce = false;
                icdFile = args[1];


                for (int i = 2; i < args.Count(); i++)
                {
                    if (args[i] == ("-ap"))
                    {
                        accessPointName = args[i + 1];

                        Console.WriteLine("Select access point " + accessPointName);

                        i++;
                    }
                    else if (args[i] == ("-ied"))
                    {
                        iedName = args[i + 1];

                        Console.WriteLine("Select IED " + iedName);

                        i++;

                    }
                    else if (args[i] == ("-out"))
                    {
                        outputFileName = args[i + 1];

                        Console.WriteLine("Select Output File " + outputFileName);

                        i++;

                    }
                    else if (args[i] == ("-modelprefix"))
                    {
                        modelPrefix = args[i + 1];

                        Console.WriteLine("Select Model Prefix " + modelPrefix);

                        i++;

                    }
                    else if (args[i] == ("-initializeonce"))
                    {
                        initializeOnce = true;

                        Console.WriteLine("Select Initialize Once");

                    }
                    else
                    {
                        Console.WriteLine("Unknown option: \"" + args[i] + "\"");
                    }
                }

                if (args[0] == "1")
                {
                    Console.WriteLine("Generate Static Model");

                    FileStream cOutStream = new FileStream(outputFileName + ".c", FileMode.Create, FileAccess.Write);
                    FileStream hOutStream = new FileStream(outputFileName + ".h", FileMode.Create, FileAccess.Write);

                    try
                    {
                        new StaticModelGenerator.StaticModelGenerator(icdFile, icdFile, cOutStream, hOutStream, outputFileName, iedName, accessPointName, modelPrefix, initializeOnce);
                    }
                    catch (Exception e)
                    {
                        Console.WriteLine("ERROR: " + e.ToString());
                    }

                }
                else if (args[0] == "2")
                {
                    Console.WriteLine("Generate Dynamic Model");

                    try
                    {
                        FileStream stream = new FileStream(outputFileName + ".cfg", FileMode.Create, FileAccess.Write);
                        new DynamicModel.DynamicModel(icdFile, stream, iedName, accessPointName);
                    }
                    catch (Exception e)
                    {
                        Console.WriteLine("ERROR: " + e.ToString());
                    }
                }
                else
                {
                    Console.WriteLine("Wrong option, parse 1 or 2 \n" +
                   "Usage: Static Model (1) \n Dynamic Model (2) \n  <generator option> <ICD file>  [-ied  <ied-name>] [-ap <access-point-name>] [-out <output-name>] [-modelprefix <model-prefix>]");
                }



            }

        }
    }
}