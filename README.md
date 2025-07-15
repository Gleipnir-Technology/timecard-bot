# Timecarder

A Matrix bot for doing timecard stuff.

## Building

On NixOS:

```
nix build .
```

This will produce files in `result/`, including `result/bin/timecardbot`

## Configuring

You must provide an environment variable `BOT_TOKEN` which contains an authentication token or a file `token.txt` which contains the token.

## Hacking

On NixOS:

```
nix develop
```

## Deployment

```
{ pkgs, lib, config, ... }:                                                                                    
let                                                                                                                     
        timecardBotSrc = pkgs.fetchFromGitHub {                                                                                            
                owner  = "Gleipnir-Technology";                                                                                        
                repo   = "timecard-bot";                                                                                                                              
                rev    = "7996c5cf18e2e12a5ea347e364a86845472df8dc";                                                           
                sha256 = "bplL0DphBwQ6xvj+t1YmDzqhbUkHzkep5enBMFjYV9w=";                                                    
        };                                                                                                                  
        timecardBotFlake = (import timecardBotSrc);                                                                             
        timecardBotPackage = timecardBotFlake.packages.${pkgs.system}.default;                                 
in                                                                                                         
{                                                                                                                                        
        environment.systemPackages = with pkgs; [                                                                                                     
                timecardBotPackage                                                                                
        ];                                                                                                                
}
```
