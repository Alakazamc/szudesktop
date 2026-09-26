$ImagePath=$env:SZU_CALENDAR_IMAGE
$ErrorActionPreference='Stop'
[Console]::OutputEncoding=[System.Text.UTF8Encoding]::new($false)
Add-Type -AssemblyName System.Runtime.WindowsRuntime
[Windows.Storage.StorageFile,Windows.Storage,ContentType=WindowsRuntime] | Out-Null
[Windows.Graphics.Imaging.BitmapDecoder,Windows.Graphics.Imaging,ContentType=WindowsRuntime] | Out-Null
[Windows.Media.Ocr.OcrEngine,Windows.Foundation,ContentType=WindowsRuntime] | Out-Null
[Windows.Globalization.Language,Windows.Globalization,ContentType=WindowsRuntime] | Out-Null
$asTask=([System.WindowsRuntimeSystemExtensions].GetMethods() | Where-Object { $_.Name -eq 'AsTask' -and $_.GetParameters().Count -eq 1 -and $_.IsGenericMethod -and $_.GetGenericArguments().Count -eq 1 -and $_.GetParameters()[0].ParameterType.Name -eq 'IAsyncOperation`1' })[0]
function Await($Operation,[Type]$ResultType){$task=$asTask.MakeGenericMethod($ResultType).Invoke($null,@($Operation));$task.Wait();$task.Result}
$file=Await ([Windows.Storage.StorageFile]::GetFileFromPathAsync($ImagePath)) ([Windows.Storage.StorageFile])
$stream=Await ($file.OpenAsync([Windows.Storage.FileAccessMode]::Read)) ([Windows.Storage.Streams.IRandomAccessStream])
$decoder=Await ([Windows.Graphics.Imaging.BitmapDecoder]::CreateAsync($stream)) ([Windows.Graphics.Imaging.BitmapDecoder])
$transform=New-Object Windows.Graphics.Imaging.BitmapTransform
$scale=[Math]::Min(1,2200.0/[Math]::Max($decoder.PixelWidth,$decoder.PixelHeight))
$transform.ScaledWidth=[uint32]($decoder.PixelWidth*$scale);$transform.ScaledHeight=[uint32]($decoder.PixelHeight*$scale)
$bitmap=Await ($decoder.GetSoftwareBitmapAsync([Windows.Graphics.Imaging.BitmapPixelFormat]::Bgra8,[Windows.Graphics.Imaging.BitmapAlphaMode]::Premultiplied,$transform,[Windows.Graphics.Imaging.ExifOrientationMode]::RespectExifOrientation,[Windows.Graphics.Imaging.ColorManagementMode]::DoNotColorManage)) ([Windows.Graphics.Imaging.SoftwareBitmap])
# 优先用简体中文识别引擎。
#
# TryCreateFromUserProfileLanguages() 跟着「当前用户的语言」走：用户语言是英文的
# 机器（云主机、英文系统、部分开发者机器）会拿英文引擎去认中文校历，得到一堆拉丁
# 乱码，正文里当然没有「校历说明」，于是被误报成「新版校历未能完整识别」——把本机
# 的语言环境问题说成学校改了格式。本机只要装了中文识别器就显式用它。
$engine=$null
foreach($tag in @('zh-Hans-CN','zh-CN','zh-Hans')){
  try{
    $lang=New-Object Windows.Globalization.Language($tag)
    $engine=[Windows.Media.Ocr.OcrEngine]::TryCreateFromLanguage($lang)
  }catch{$engine=$null}
  if($null -ne $engine){break}
}
if($null -eq $engine){$engine=[Windows.Media.Ocr.OcrEngine]::TryCreateFromUserProfileLanguages()}
if($null -eq $engine){throw 'OCR language pack unavailable'}
$result=Await ($engine.RecognizeAsync($bitmap)) ([Windows.Media.Ocr.OcrResult])
$result.Lines | ForEach-Object {$_.Text}
$stream.Dispose();$bitmap.Dispose()
