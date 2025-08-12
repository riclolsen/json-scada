This tool can be accessed from both the command line of visual studio. 
To run the tool from the command line a command of the following format has to be generated:

<generator option> <ICD file>  -ied  <ied-name> -ap <access-point-name> -out <output-name> -modelprefix <model-prefix>

Usage:
Static Model (1)
Dynamic Model (2)

The values in <> have to be replaced with the values corresponding to an arbitrary ICD file. 
To run this command completely the command should look like this: 

Example: 

dotnet Tools.dll 1 ICDFiles/genericIO.icd -ied simpleIO -ap accessPoint1 -out static_model -modelprefix iedModel