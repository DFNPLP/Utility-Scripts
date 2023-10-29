 param (
    [string]$srcDrive="",
    [string]$dest = ""
 )

#need to do a path split here
#$srcDrive = Resolve-Path -Path $srcDrive
#$dest = Resolve-Path -Path $dest
# need to escape some strings, as the \\<nameofnetworkresource>\<name> Folder\initial_test seems to be invalid because of the space in it

#todo, might be able to set password?
# worked with ./backup_now.ps1 "c:,d:" "\\<nameofnetworkresource>\<name> Folder\initial_test" ????
wbadmin start backup -allCritical -quiet -backupTarget:$dest