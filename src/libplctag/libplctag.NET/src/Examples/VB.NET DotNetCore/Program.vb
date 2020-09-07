Imports System.Net
Imports System.Threading
Imports libplctag
Imports libplctag.DataTypes

Module Module1

    Sub Main()

        Dim myTag = New Tag(Of DintPlcMapper, Integer)() With
        {
            .Name = "PROGRAM:SomeProgram.SomeDINT",
            .Gateway = "10.10.10.10",
            .PlcType = PlcType.ControlLogix,
            .Path = "1,0",
            .Timeout = TimeSpan.FromMilliseconds(5000)
        }
        myTag.Initialize()

        myTag.Value = 3737
        myTag.Write()

        myTag.Read()
        Dim myDint = myTag.Value

        Console.WriteLine(myDint)
        Console.ReadKey()

    End Sub

End Module