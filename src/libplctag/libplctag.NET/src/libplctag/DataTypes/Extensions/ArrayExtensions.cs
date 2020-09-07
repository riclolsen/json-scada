using System;
using System.Collections.Generic;
using System.Text;

namespace libplctag.DataTypes.Extensions
{
    public static class ArrayExtensions
    {
        /// <summary>
        /// Extension method to flatten a 2D array to a 1D array
        /// </summary>
        /// <typeparam name="T">Array Type</typeparam>
        /// <param name="input">2D array to be flattened</param>
        /// <returns>1D array</returns>
        public static T[] To1DArray<T>(this T[,] input)
        {
            // Step 1: get total size of 2D array, and allocate 1D array.
            int size = input.Length;
            T[] result = new T[size];

            // Step 2: copy 2D array elements into a 1D array.
            int write = 0;
            for (int i = 0; i <= input.GetUpperBound(0); i++)
            {
                for (int z = 0; z <= input.GetUpperBound(1); z++)
                {
                    result[write++] = input[i, z];
                }
            }
            // Step 3: return the new array.
            return result;
        }

        /// <summary>
        /// Extension method to flatten a 3D array to a 1D array
        /// </summary>
        /// <typeparam name="T">Array Type</typeparam>
        /// <param name="input">3D array to be flattened</param>
        /// <returns>1D array</returns>
        public static T[] To1DArray<T>(this T[,,] input)
        {
            // Step 1: get total size of 3D array, and allocate 1D array.
            int size = input.Length;
            T[] result = new T[size];

            // Step 2: copy 3D array elements into a 1D array.
            int write = 0;
            for (int i = 0; i <= input.GetUpperBound(0); i++)
            {
                for (int j = 0; j <= input.GetUpperBound(1); j++)
                {
                    for (int k = 0; k < input.GetUpperBound(2); k++)
                    {
                        result[write++] = input[i, j, k];
                    }
                }
            }
            // Step 3: return the new array.
            return result;
        }

        /// <summary>
        /// Extension method to reshape a 1D array into a 2D array
        /// </summary>
        /// <typeparam name="T">Array Type</typeparam>
        /// <param name="input">1D array to be reshaped</param>
        /// <param name="height">Desired height (first index) of 2D array</param>
        /// <param name="width">Desired width (second index) of 2D array</param>
        /// <returns>2D array</returns>
        public static T[,] To2DArray<T>(this T[] input, int height, int width)
        {
            T[,] output = new T[height, width];

            for (int i = 0; i < height; i++)
            {
                for (int j = 0; j < width; j++)
                {
                    output[i, j] = input[i * width + j];
                }
            }
            return output;
        }

        /// <summary>
        /// Extension method to reshape a 1D array into a 3D array
        /// </summary>
        /// <typeparam name="T">Array Type</typeparam>
        /// <param name="input">1D array to be reshaped</param>
        /// <param name="height">Desired height (first index) of 3D array</param>
        /// <param name="width">Desired width (second index) of 3D array</param>
        /// <param name="length">Desired length (third index) of 3D array</param>
        /// <returns>#D array</returns>
        public static T[,,] To3DArray<T>(this T[] input, int height, int width, int length)
        {
            T[,,] output = new T[height, width, length];

            for (int i = 0; i < height; i++)
            {
                for (int j = 0; j < width; j++)
                {
                    for (int k = 0; k < length; k++)
                    {
                        output[i, j, k] = input[i * height * width + j * width + k];
                    }
                }
            }
            return output;
        }

    }
}
