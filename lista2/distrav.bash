#!/bin/bash

FILE=${1? 'parameter ${1} should be the name of the travel history file'}

TAB=()
while IFS= read -r line; do
  TAB+=("$line")
done < <(sort -n -k1 "$FILE")

TAB_LENGTH=${#TAB[@]}

# echo "TAB_LENGTH = "${TAB_LENGTH} ## TEST

read -a ARGS < <(echo ${TAB[0]});

if (( ARGS[0] != -1 ));
then
  echo 'The first line should start with -1 and then contain display parameters';
  exit -1;
fi;

declare -i TRAVELERS=${ARGS[1]:? 'number of travelers missing'}
declare -i WIDTH=${ARGS[2]:? 'display width missing'}
declare -i HEIGHT=${ARGS[3]:? 'display height missing'}
declare -i EMPTY_ID=$((TRAVELERS + 100))  # Define a separate ID for empty spaces
declare -a DISPLAY
declare -a LAST_X
declare -a LAST_Y
declare -a BELOW # BELOW[${ID}] is covered by ${ID}
declare -a ABOVE # ABOVE[${ID}] ABOVE ${ID}
declare -a SYMBOL_ID # SYMBOL ASSIGNED TO ${ID}
declare -i START_STEP=1 # The first step in 'travel history'

# init empty DISPLAY

function d_idx {  # index in DISPLAY for arguments: x y - coordimates
  X=${1}
  Y=${2}
  echo $(( Y * WIDTH + X )) 
  # echo $X $Y
}

function h_line {
    echo -n '+'
    for (( X=0; X < WIDTH; X++ ))
    do
      echo -n "--+";
    done;
    echo
}

H_LINE=$(h_line);

function line_y {
    Y=${1}
    # Exactly match original format - each cell has a '.' prefix
    for (( X=0; X < WIDTH; X++ ))
    do
      ID=${DISPLAY[$(d_idx $X $Y)]}
      if (( ID == EMPTY_ID )); then
        echo -n "..";  # For empty spaces, print '..'
      else
        echo -n ".${SYMBOL_ID[${ID}]}";  # For occupied spaces, print '.X' where X is the symbol
      fi
    done;
    echo
}

declare -a LINE_Y;


function display_reset { # reset the display
  DISPLAY[$(d_idx WIDTH HEIGHT)]=${EMPTY_ID} # space on the 'hidden' position
  # clean screen
  for (( Y=0; Y < HEIGHT; Y++ )) 
  do
    for (( X=0; X < WIDTH; X++ ))
    do
      DISPLAY[$(d_idx $X $Y)]=${EMPTY_ID}; # ${EMPTY_ID} is ID of empty space
    done;
  done;
  
  # Initially, all the travelers are on the 'hidden' position
  for (( ID=0; ID < TRAVELERS; ID++ ))
  do
    ABOVE[${ID}]=${EMPTY_ID};
    BELOW[${ID}]=${EMPTY_ID};
    LAST_X[${ID}]=${WIDTH};
    LAST_Y[${ID}]=${HEIGHT};
    SYMBOL_ID[${ID}]='?';
  done;    
  SYMBOL_ID[${EMPTY_ID}]='.' # ${EMPTY_ID} is ID of empty space

  for (( Y=0; Y < HEIGHT; Y++ )) 
  do
    LINE_Y[${Y}]=$(line_y ${Y});
  done;

}

declare -i STEP=0
function display_print { # print current display

  clear;
  echo "STEP = ${STEP}  TIME = ${ARGS[0]}" 

  for (( Y=0; Y < HEIGHT; Y++ )) 
  do
    # echo ${H_LINE};
    echo ${LINE_Y[${Y}]};
  done;
  # echo ${H_LINE};
}


# exit #####

function display_update_by_step { # $1 is step number
  STEP=${1}
  read -a ARGS < <(echo ${TAB[${STEP}]});
  # echo ${!ar[@]} ${ar[@]};  ## TEST
  # echo ${ARGS[@]};
  # echo "STEP = ${STEP}  TIME = ${ARGS[0]}" 
  
  ID=${ARGS[1]}
  X=${ARGS[2]}
  Y=${ARGS[3]}
  SYMBOL=${ARGS[4]}
  
  # Initialize arrays for new IDs (tenants)
  if [[ -z ${ABOVE[${ID}]} ]]; then
    ABOVE[${ID}]=${EMPTY_ID}
    BELOW[${ID}]=${EMPTY_ID}
    LAST_X[${ID}]=${WIDTH}
    LAST_Y[${ID}]=${HEIGHT}
  fi
  
  SYMBOL_ID[${ID}]=${SYMBOL}
  if (( ${ABOVE[${ID}]} == ${EMPTY_ID} )) # if empty space ABOVE ${ID} ...
  then
    DISPLAY[$(d_idx ${LAST_X[${ID}]} ${LAST_Y[${ID}]})]=${BELOW[${ID}]} # then uncover the lower occupant
  fi
  # relations bypasss the  ${ID}
  ABOVE[${BELOW[${ID}]}]=${ABOVE[${ID}]};
  BELOW[${ABOVE[${ID}]}]=${BELOW[${ID}]};
  
  # Rebuild the line where entity was previously
  if (( Y != LAST_Y[${ID}] && LAST_Y[${ID}] < HEIGHT ))
  then
    LINE_Y[${LAST_Y[${ID}]}]=$(line_y ${LAST_Y[${ID}]})
  fi
  
  # new 'last' coordinates
  LAST_X[${ID}]=${X}
  LAST_Y[${ID}]=${Y}
  # cover new position:
  BELOW[${ID}]=${DISPLAY[$(d_idx ${LAST_X[${ID}]} ${LAST_Y[${ID}]})]}
  ABOVE[${DISPLAY[$(d_idx ${LAST_X[${ID}]} ${LAST_Y[${ID}]})]}]=${ID}
  # replace on display
  ABOVE[${ID}]=${EMPTY_ID} # ${ID} is on top
  DISPLAY[$(d_idx ${LAST_X[${ID}]} ${LAST_Y[${ID}]})]=${ID} # ocupy the new position  
  
  # Rebuild the line where entity moved to
  LINE_Y[${LAST_Y[${ID}]}]=$(line_y ${LAST_Y[${ID}]})
}


# ACTION !

clear;
echo "START"
display_reset; # test reset
display_print; # test display printing
echo -n "TO STEP? "
read TO_STEP

for (( STEP=START_STEP; STEP< TAB_LENGTH; STEP++ ))
do
  display_update_by_step ${STEP} ;
  display_print ;
  if (( TO_STEP <= STEP  ));
  then
    echo -n "TO STEP? "
    read TO_STEP
  fi;
done