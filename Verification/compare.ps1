 param (
    [string]$src = "",
    [string]$dest = "",
	[switch]$r = $false
 )
	
#resolve potentially relative path to absolute one
$src = Resolve-Path -Path $src
$dest = Resolve-Path -Path $dest
#strip final backslash if one is present:
$src = ($src -replace "\\$", "")
$dest = ($dest -replace "\\$", "")

$fileCountOk = 0
$fileCountHashMismatch = 0
$fileCountOtherError = 0
$sourceData = $null
if ($r) {
	$sourceData = Get-ChildItem -Recurse $src | Where { ! $_.PSIsContainer } | Select Name,FullName,Extension,BaseName
} else {
	$sourceData = Get-ChildItem $src | Where { ! $_.PSIsContainer } | Select Name,FullName,Extension,BaseName
}

$sourceData | Foreach-Object {
	$fileName = $_.Name
	$fileNameNoExt = $_.BaseName
	$fileExt = $_.Extension
	
	$fullPath = $_.FullName
	$pathFromRoot = ($fullPath -replace [regex]::Escape("$src\"), "")
	$regexForStrippingFileName = "\\?" + [regex]::Escape("$fileName") + "$"
	$relativePathFromRoot = ($pathFromRoot -replace $regexForStrippingFileName, "")
	
	$destFileRegex = [regex]::Escape("$fileNameNoExt") + "( \(\d{4}_\d{2}_\d{2} \d{2}_\d{2}_\d{2} UTC\))?" + [regex]::Escape("$fileExt")
	$fileToCheck = Get-ChildItem -Path "$dest\$relativePathFromRoot\" | Where-Object { $_.Name -match $destFileRegex}

	# Get the file hashes
	$hashSrc = Get-FileHash $fullPath -Algorithm "SHA256"

	Try
	{	
		$hashDest = Get-FileHash $fileToCheck.FullName -Algorithm "SHA256"

		# Hash comparison
		if ($hashSrc.Hash -ne $hashDest.Hash)
		{
			Write-Output "ERR	Hash Mismatch   	$fullPath	$hashSrc	$hashDest"
			$fileCountHashMismatch = $fileCountHashMismatch + 1
		} else {
			Write-Output "OK	                	$fullPath"
			$fileCountOk = $fileCountOk + 1
		}
	}
	Catch
	{
		Write-Output "ERR	Other Error    	$fullPath	$hashSrc"
		$fileCountOtherError = $fileCountOtherError + 1
	}
}

Write-Output ""
Write-Output ""
Write-Output "File Count OK: $fileCountOk"
Write-Output "File Count Hash Mismatch: $fileCountHashMismatch"
Write-Output "File Count Other Error: $fileCountOtherError"
