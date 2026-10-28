using System;
using System.Collections.Generic;
using System.Runtime.InteropServices;
namespace Leprechaun {
[ComImport,Guid("BCDE0395-E52F-467C-8E3D-C4579291692E")] class Enumerator {}
[ComImport,Guid("A95664D2-9614-4F35-A746-DE8DB63617E6"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IEnum {
 [PreserveSig]int EnumAudioEndpoints(int flow,uint state,out ICollection devices);
 [PreserveSig]int GetDefaultAudioEndpoint(int flow,int role,out IDevice device);
 [PreserveSig]int GetDevice([MarshalAs(UnmanagedType.LPWStr)]string id,out IDevice device);
 [PreserveSig]int RegisterEndpointNotificationCallback(IntPtr p); [PreserveSig]int UnregisterEndpointNotificationCallback(IntPtr p);
}
[ComImport,Guid("0BD7A1BE-7A1A-44DB-8397-CC5392387B5E"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface ICollection {
 [PreserveSig]int GetCount(out uint count);[PreserveSig]int Item(uint i,out IDevice device);
}
[ComImport,Guid("D666063F-1587-4E43-81F1-B948E807363F"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IDevice {
 [PreserveSig]int Activate(ref Guid iid,uint context,IntPtr args,[MarshalAs(UnmanagedType.IUnknown)]out object result);
 [PreserveSig]int OpenPropertyStore(uint access,out IStore store);
 [PreserveSig]int GetId([MarshalAs(UnmanagedType.LPWStr)]out string id);[PreserveSig]int GetState(out uint state);
}
[ComImport,Guid("1BE09788-6894-4089-8586-9A2A6C265AC5"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IEndpoint {[PreserveSig]int GetDataFlow(out int flow);}
[StructLayout(LayoutKind.Sequential)] struct Key {public Guid fmtid;public uint pid;}
[StructLayout(LayoutKind.Explicit,Size=24)] struct Variant {[FieldOffset(0)]public ushort vt;[FieldOffset(8)]public IntPtr value;}
[ComImport,Guid("886D8EEB-8CF2-4446-8D02-CDBA1DBDCF99"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IStore {
 [PreserveSig]int GetCount(out uint n);[PreserveSig]int GetAt(uint i,out Key key);[PreserveSig]int GetValue(ref Key key,out Variant value);
 [PreserveSig]int SetValue(ref Key k,ref Variant v);[PreserveSig]int Commit();
}
[ComImport,Guid("5CDF2C82-841E-4546-9722-0CF74078229A"),InterfaceType(ComInterfaceType.InterfaceIsIUnknown)] interface IVolume {
 [PreserveSig]int RegisterControlChangeNotify(IntPtr p);[PreserveSig]int UnregisterControlChangeNotify(IntPtr p);[PreserveSig]int GetChannelCount(out uint n);
 [PreserveSig]int SetMasterVolumeLevel(float v,ref Guid g);[PreserveSig]int SetMasterVolumeLevelScalar(float v,ref Guid g);
 [PreserveSig]int GetMasterVolumeLevel(out float v);[PreserveSig]int GetMasterVolumeLevelScalar(out float v);
 [PreserveSig]int SetChannelVolumeLevel(uint c,float v,ref Guid g);[PreserveSig]int SetChannelVolumeLevelScalar(uint c,float v,ref Guid g);
 [PreserveSig]int GetChannelVolumeLevel(uint c,out float v);[PreserveSig]int GetChannelVolumeLevelScalar(uint c,out float v);
 [PreserveSig]int SetMute([MarshalAs(UnmanagedType.Bool)]bool v,ref Guid g);[PreserveSig]int GetMute([MarshalAs(UnmanagedType.Bool)]out bool v);
}
public class DeviceInfo {public string id;public string name;public string flow;}
public static class Audio {
 [DllImport("ole32.dll")]static extern int PropVariantClear(ref Variant v);
 static void Check(int hr){Marshal.ThrowExceptionForHR(hr);}
 public static DeviceInfo[] List(){var result=new List<DeviceInfo>();var e=(IEnum)new Enumerator();try{for(int flow=0;flow<2;flow++){ICollection c;Check(e.EnumAudioEndpoints(flow,1,out c));try{uint n;Check(c.GetCount(out n));for(uint i=0;i<n;i++){IDevice d;Check(c.Item(i,out d));try{string id;Check(d.GetId(out id));IStore store;Check(d.OpenPropertyStore(0,out store));string name=id;try{var key=new Key{fmtid=new Guid("A45C254E-DF1C-4EFD-8020-67D146A850E0"),pid=14};Variant v;Check(store.GetValue(ref key,out v));try{if(v.vt==31)name=Marshal.PtrToStringUni(v.value);}finally{PropVariantClear(ref v);}}finally{Marshal.ReleaseComObject(store);}result.Add(new DeviceInfo{id=id,name=name,flow=flow==0?"render":"capture"});}finally{Marshal.ReleaseComObject(d);}}}finally{Marshal.ReleaseComObject(c);}}}finally{Marshal.ReleaseComObject(e);}return result.ToArray();}
 public static void Apply(string id,bool capture,string action,double amount){var e=(IEnum)new Enumerator();IDevice d=null;object obj=null;try{if(String.IsNullOrEmpty(id)||id=="default")Check(e.GetDefaultAudioEndpoint(capture?1:0,1,out d));else Check(e.GetDevice(id,out d));int flow;Check(((IEndpoint)d).GetDataFlow(out flow));if(flow!=(capture?1:0))throw new InvalidOperationException("Selected device has the wrong render/capture direction");Guid iid=typeof(IVolume).GUID;Check(d.Activate(ref iid,23,IntPtr.Zero,out obj));var volume=(IVolume)obj;Guid context=Guid.Empty;if(action.EndsWith("mute")){bool muted;Check(volume.GetMute(out muted));Check(volume.SetMute(!muted,ref context));}else{float level;Check(volume.GetMasterVolumeLevelScalar(out level));double target=action.EndsWith("set")?amount/100.0:level+(action.EndsWith("down")?-amount:amount)/100.0;Check(volume.SetMasterVolumeLevelScalar((float)Math.Max(0,Math.Min(1,target)),ref context));}}finally{if(obj!=null)Marshal.ReleaseComObject(obj);if(d!=null)Marshal.ReleaseComObject(d);Marshal.ReleaseComObject(e);}}
}
}
