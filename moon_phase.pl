#!/usr/bin/perl

use warnings;
use strict;

use Astro::MoonPhase qw{phase};
use Sys::Uptime;

my $uptime = sprintf("%d", Sys::Uptime::uptime());
my $delta = time() - $uptime;

my ( $MoonPhase,
     $MoonIllum,
     $MoonAge,
     $MoonDist,
     $MoonAng,
     $SunDist,
     $SunAng ) = phase($delta);

printf("%.2f\n", $MoonPhase);
